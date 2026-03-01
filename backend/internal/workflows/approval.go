package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/sakeththota/durable-embedding-migration/backend/internal/activities"
)

const (
	ApprovalTaskQueue   = "approval-migration"
	SignalApprove       = "approve"
	SignalReject        = "reject"
	SignalUpdateParams  = "update-params"
	QueryApprovalStatus = "approval-status"
)

type ApprovalMigrationInput struct {
	Version          string
	ModelName        string
	Dimensions       int
	BatchSize        int
	ApprovalTimeout  time.Duration
	ProcessedRecords int
	TotalRecords     int
}

type ApprovalMigrationUpdate struct {
	BatchSize int `json:"batch_size"`
}

type ApprovalMigrationProgress struct {
	Status           string    `json:"status"`
	Version          string    `json:"version"`
	ModelName        string    `json:"model_name"`
	TotalRecords     int       `json:"total_records"`
	ProcessedRecords int       `json:"processed_records"`
	CurrentBatch     int       `json:"current_batch"`
	PendingUpdate    string    `json:"pending_update,omitempty"`
	AwaitingApproval bool      `json:"awaiting_approval"`
	Approved         bool      `json:"approved"`
	Rejected         bool      `json:"rejected"`
	StartedAt        time.Time `json:"started_at"`
	LastActivityAt   time.Time `json:"last_activity_at"`
	CompletedAt      time.Time `json:"completed_at,omitempty"`
}

func ApprovalMigrationWorkflow(ctx workflow.Context, input ApprovalMigrationInput) error {
	logger := workflow.GetLogger(ctx)

	if input.BatchSize <= 0 {
		input.BatchSize = 10
	}
	if input.ApprovalTimeout <= 0 {
		input.ApprovalTimeout = 1 * time.Hour
	}

	progress := ApprovalMigrationProgress{
		Status:           "in_progress",
		Version:          input.Version,
		ModelName:        input.ModelName,
		ProcessedRecords: input.ProcessedRecords,
		TotalRecords:     input.TotalRecords,
		StartedAt:        workflow.Now(ctx),
		LastActivityAt:   workflow.Now(ctx),
		AwaitingApproval: false,
		Approved:         false,
		Rejected:         false,
	}

	pendingUpdate := ""

	if err := workflow.SetQueryHandler(ctx, QueryApprovalStatus, func() (ApprovalMigrationProgress, error) {
		progress.PendingUpdate = pendingUpdate
		return progress, nil
	}); err != nil {
		return err
	}

	approveCh := workflow.GetSignalChannel(ctx, SignalApprove)
	rejectCh := workflow.GetSignalChannel(ctx, SignalReject)
	updateCh := workflow.GetSignalChannel(ctx, SignalUpdateParams)

	drainSignals := func() {
		for {
			var signal interface{}
			ok := updateCh.ReceiveAsync(&signal)
			if !ok {
				break
			}
			if update, ok := signal.(ApprovalMigrationUpdate); ok {
				input.BatchSize = update.BatchSize
				pendingUpdate = "batch_size updated to " + string(rune(update.BatchSize+'0'))
				logger.Info("received update signal", "batchSize", update.BatchSize)
			}
		}
		for {
			ok := approveCh.ReceiveAsync(nil)
			if !ok {
				break
			}
			progress.Approved = true
			progress.AwaitingApproval = false
			logger.Info("received approve signal")
		}
		for {
			ok := rejectCh.ReceiveAsync(nil)
			if !ok {
				break
			}
			progress.Rejected = true
			progress.AwaitingApproval = false
			logger.Info("received reject signal")
		}
	}

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	})

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

	for progress.ProcessedRecords < progress.TotalRecords {
		drainSignals()

		if progress.Rejected {
			logger.Info("migration rejected, cleaning up")
			progress.Status = "rejected"

			_ = workflow.ExecuteActivity(actCtx, "DeleteEmbeddings", activities.DeleteEmbeddingsInput{
				Version: input.Version,
			}).Get(ctx, nil)

			_ = workflow.ExecuteActivity(actCtx, "UpdateProgress", activities.UpdateProgressInput{
				Version:          input.Version,
				ProcessedRecords: 0,
			}).Get(ctx, nil)

			return nil
		}

		progress.CurrentBatch++
		logger.Info("processing batch", "batch", progress.CurrentBatch, "batchSize", input.BatchSize)

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
		progress.LastActivityAt = workflow.Now(ctx)
		pendingUpdate = ""

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

		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			logger.Info("continuing as new workflow", "processed", progress.ProcessedRecords)
			return workflow.NewContinueAsNewError(ctx, ApprovalMigrationWorkflow, ApprovalMigrationInput{
				Version:          input.Version,
				ModelName:        input.ModelName,
				Dimensions:       input.Dimensions,
				BatchSize:        input.BatchSize,
				ApprovalTimeout:  input.ApprovalTimeout,
				ProcessedRecords: progress.ProcessedRecords,
				TotalRecords:     progress.TotalRecords,
			})
		}
	}

	err := workflow.ExecuteActivity(actCtx, "ValidateMigration", activities.ValidateMigrationInput{
		Version:      input.Version,
		TotalRecords: progress.TotalRecords,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	logger.Info("migration generation complete, waiting for approval", "version", input.Version)
	progress.Status = "awaiting_approval"
	progress.AwaitingApproval = true

	var timerFired bool
	approvalTimer := workflow.NewTimer(ctx, input.ApprovalTimeout)

	for {
		sel := workflow.NewSelector(ctx)
		sel.AddReceive(approveCh, func(ch workflow.ReceiveChannel, more bool) {
			ch.Receive(ctx, nil)
			progress.Approved = true
			progress.AwaitingApproval = false
			logger.Info("approval received")
		})
		sel.AddReceive(rejectCh, func(ch workflow.ReceiveChannel, more bool) {
			ch.Receive(ctx, nil)
			progress.Rejected = true
			progress.AwaitingApproval = false
			logger.Info("rejection received")
		})
		sel.AddReceive(updateCh, func(ch workflow.ReceiveChannel, more bool) {
			var signal ApprovalMigrationUpdate
			ch.Receive(ctx, &signal)
			input.BatchSize = signal.BatchSize
			pendingUpdate = "batch_size updated to " + string(rune(signal.BatchSize+'0'))
			logger.Info("update signal received during approval", "batchSize", signal.BatchSize)
		})
		sel.AddFuture(approvalTimer, func(f workflow.Future) {
			timerFired = true
			logger.Info("approval timeout reached")
		})
		sel.Select(ctx)

		if progress.Approved || progress.Rejected {
			break
		}

		if timerFired {
			logger.Info("approval timeout, auto-rejecting")
			progress.Status = "timeout"
			progress.Rejected = true
			break
		}

		approvalTimer = workflow.NewTimer(ctx, input.ApprovalTimeout)
		timerFired = false
	}

	if progress.Rejected {
		logger.Info("rolling back migration")
		_ = workflow.ExecuteActivity(actCtx, "DeleteEmbeddings", activities.DeleteEmbeddingsInput{
			Version: input.Version,
		}).Get(ctx, nil)

		_ = workflow.ExecuteActivity(actCtx, "UpdateProgress", activities.UpdateProgressInput{
			Version:          input.Version,
			ProcessedRecords: 0,
		}).Get(ctx, nil)

		progress.Status = "cancelled"
		progress.CompletedAt = workflow.Now(ctx)
		return nil
	}

	err = workflow.ExecuteActivity(actCtx, "SwitchActiveVersion", activities.SwitchActiveVersionInput{
		Version: input.Version,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	progress.Status = "completed"
	progress.AwaitingApproval = false
	progress.CompletedAt = workflow.Now(ctx)
	logger.Info("migration completed and approved", "version", input.Version)
	return nil
}
