package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/activities"
)

const (
	TaskQueue     = "embedding-migration"
	SignalPause   = "pause"
	SignalResume  = "resume"
	QueryProgress = "progress"
)

// MigrationInput is the workflow input.
type MigrationInput struct {
	Version          string
	ModelName        string
	Dimensions       int
	BatchSize        int
	ProcessedRecords int // non-zero when resuming via continue-as-new
	TotalRecords     int // non-zero when resuming via continue-as-new
}

// MigrationProgress is returned by the progress query.
type MigrationProgress struct {
	Status           string    `json:"status"`
	TotalRecords     int       `json:"total_records"`
	ProcessedRecords int       `json:"processed_records"`
	CurrentBatch     int       `json:"current_batch"`
	StartedAt        time.Time `json:"started_at"`
	LastActivityAt   time.Time `json:"last_activity_at"`
}

// EmbeddingMigrationWorkflow orchestrates the re-embedding of all hotels
// with a new model. Supports pause/resume signals and progress queries.
func EmbeddingMigrationWorkflow(ctx workflow.Context, input MigrationInput) error {
	logger := workflow.GetLogger(ctx)

	if input.BatchSize <= 0 {
		input.BatchSize = 10
	}

	// Workflow-local state for progress queries.
	progress := MigrationProgress{
		Status:           "in_progress",
		ProcessedRecords: input.ProcessedRecords,
		TotalRecords:     input.TotalRecords,
		StartedAt:        workflow.Now(ctx),
		LastActivityAt:   workflow.Now(ctx),
	}
	paused := false

	// Register progress query handler.
	if err := workflow.SetQueryHandler(ctx, QueryProgress, func() (MigrationProgress, error) {
		return progress, nil
	}); err != nil {
		return err
	}

	// Set up signal channels for pause/resume.
	pauseCh := workflow.GetSignalChannel(ctx, SignalPause)
	resumeCh := workflow.GetSignalChannel(ctx, SignalResume)

	// Drain any pending signals (non-blocking).
	drainSignals := func() {
		for {
			ok := pauseCh.ReceiveAsync(nil)
			if !ok {
				break
			}
			paused = true
		}
		for {
			ok := resumeCh.ReceiveAsync(nil)
			if !ok {
				break
			}
			paused = false
		}
	}

	// Activity options with retry policy.
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		HeartbeatTimeout:    5 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Second,
		},
	})

	// Step 1: Initialize migration (only on first run, not continue-as-new).
	if input.ProcessedRecords == 0 {
		var initResult activities.InitMigrationResult
		err := workflow.ExecuteActivity(actCtx, "InitMigration", activities.InitMigrationInput{
			Version:    input.Version,
			ModelName:  input.ModelName,
			Dimensions: input.Dimensions,
		}).Get(ctx, &initResult)
		if err != nil {
			return err
		}

		progress.TotalRecords = initResult.TotalRecords
		input.TotalRecords = initResult.TotalRecords
		logger.Info("migration initialized", "total", initResult.TotalRecords)
	}

	// Step 2: Process batches.
	for progress.ProcessedRecords < progress.TotalRecords {
		// Check for pause/resume signals.
		drainSignals()

		// If paused, block until resume.
		if paused {
			progress.Status = "paused"
			logger.Info("migration paused", "processed", progress.ProcessedRecords)

			// Block on resume signal, but also check pause in case of duplicate signals.
			for paused {
				sel := workflow.NewSelector(ctx)
				sel.AddReceive(resumeCh, func(ch workflow.ReceiveChannel, more bool) {
					ch.Receive(ctx, nil)
					paused = false
				})
				sel.AddReceive(pauseCh, func(ch workflow.ReceiveChannel, more bool) {
					ch.Receive(ctx, nil)
					paused = true
				})
				sel.Select(ctx)
			}

			progress.Status = "in_progress"
			logger.Info("migration resumed", "processed", progress.ProcessedRecords)
		}

		// Fetch batch.
		var fetchResult activities.FetchBatchResult
		err := workflow.ExecuteActivity(actCtx, "FetchBatch", activities.FetchBatchInput{
			Offset: progress.ProcessedRecords,
			Limit:  input.BatchSize,
		}).Get(ctx, &fetchResult)
		if err != nil {
			return err
		}

		if len(fetchResult.Hotels) == 0 {
			break
		}

		// Generate embeddings and store.
		var storeResult activities.GenerateAndStoreResult
		err = workflow.ExecuteActivity(actCtx, "GenerateAndStoreEmbeddings", activities.GenerateAndStoreInput{
			Hotels:    fetchResult.Hotels,
			Version:   input.Version,
			ModelName: input.ModelName,
		}).Get(ctx, &storeResult)
		if err != nil {
			return err
		}

		progress.ProcessedRecords += storeResult.Processed
		progress.CurrentBatch++
		progress.LastActivityAt = workflow.Now(ctx)

		// Update progress in DB.
		err = workflow.ExecuteActivity(actCtx, "UpdateProgress", activities.UpdateProgressInput{
			Version:          input.Version,
			ProcessedRecords: progress.ProcessedRecords,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

		logger.Info("batch complete",
			"batch", progress.CurrentBatch,
			"processed", progress.ProcessedRecords,
			"total", progress.TotalRecords,
		)

		// Check if we should continue-as-new to avoid history bloat.
		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			logger.Info("continuing as new workflow", "processed", progress.ProcessedRecords)
			return workflow.NewContinueAsNewError(ctx, EmbeddingMigrationWorkflow, MigrationInput{
				Version:          input.Version,
				ModelName:        input.ModelName,
				Dimensions:       input.Dimensions,
				BatchSize:        input.BatchSize,
				ProcessedRecords: progress.ProcessedRecords,
				TotalRecords:     progress.TotalRecords,
			})
		}
	}

	// Step 3: Validate migration.
	err := workflow.ExecuteActivity(actCtx, "ValidateMigration", activities.ValidateMigrationInput{
		Version:      input.Version,
		TotalRecords: progress.TotalRecords,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Step 4: Switch active version.
	err = workflow.ExecuteActivity(actCtx, "SwitchActiveVersion", activities.SwitchActiveVersionInput{
		Version: input.Version,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	progress.Status = "completed"
	logger.Info("migration completed", "version", input.Version)
	return nil
}
