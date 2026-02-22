package activities

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/db"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/embeddings"
)

// Activities holds the dependencies needed by all activity functions.
type Activities struct {
	Pool   *pgxpool.Pool
	Ollama *embeddings.Client
}

// InitMigrationInput is the input for InitMigration.
type InitMigrationInput struct {
	Version    string
	ModelName  string
	Dimensions int
}

// InitMigrationResult is the output of InitMigration.
type InitMigrationResult struct {
	TotalRecords int
}

// InitMigration creates the version record and returns the total hotel count.
func (a *Activities) InitMigration(ctx context.Context, input InitMigrationInput) (InitMigrationResult, error) {
	count, err := db.CountHotels(ctx, a.Pool)
	if err != nil {
		return InitMigrationResult{}, fmt.Errorf("counting hotels: %w", err)
	}

	if err := db.CreateVersion(ctx, a.Pool, input.Version, input.ModelName, input.Dimensions, count); err != nil {
		return InitMigrationResult{}, fmt.Errorf("creating version: %w", err)
	}

	slog.Info("initialized migration", "version", input.Version, "model", input.ModelName, "total", count)
	return InitMigrationResult{TotalRecords: count}, nil
}

// FetchBatchInput is the input for FetchBatch.
type FetchBatchInput struct {
	Offset int
	Limit  int
}

// HotelRecord is a minimal hotel representation passed between activities.
type HotelRecord struct {
	ID   string
	Text string // name + " " + description
}

// FetchBatchResult is the output of FetchBatch.
type FetchBatchResult struct {
	Hotels []HotelRecord
}

// FetchBatch retrieves a page of hotels for embedding.
func (a *Activities) FetchBatch(ctx context.Context, input FetchBatchInput) (FetchBatchResult, error) {
	hotels, err := db.GetHotelBatch(ctx, a.Pool, input.Offset, input.Limit)
	if err != nil {
		return FetchBatchResult{}, fmt.Errorf("fetching batch: %w", err)
	}

	records := make([]HotelRecord, len(hotels))
	for i, h := range hotels {
		records[i] = HotelRecord{
			ID:   h.ID,
			Text: h.Name + " " + h.Description,
		}
	}

	return FetchBatchResult{Hotels: records}, nil
}

// GenerateAndStoreInput is the input for GenerateAndStoreEmbeddings.
type GenerateAndStoreInput struct {
	Hotels    []HotelRecord
	Version   string
	ModelName string
}

// GenerateAndStoreResult is the output of GenerateAndStoreEmbeddings.
type GenerateAndStoreResult struct {
	Processed int
}

// GenerateAndStoreEmbeddings embeds each hotel and saves to the database.
// Includes a 5% random failure rate to demonstrate Temporal retries.
func (a *Activities) GenerateAndStoreEmbeddings(ctx context.Context, input GenerateAndStoreInput) (GenerateAndStoreResult, error) {
	// 5% failure injection for demo purposes.
	if rand.Float64() < 0.05 {
		return GenerateAndStoreResult{}, fmt.Errorf("simulated transient failure (5%% chance)")
	}

	for _, hotel := range input.Hotels {
		embedding, err := a.Ollama.Embed(ctx, input.ModelName, hotel.Text)
		if err != nil {
			return GenerateAndStoreResult{}, fmt.Errorf("embedding hotel %s: %w", hotel.ID, err)
		}

		if err := db.SaveEmbedding(ctx, a.Pool, hotel.ID, input.Version, input.ModelName, len(embedding), embedding); err != nil {
			return GenerateAndStoreResult{}, fmt.Errorf("saving embedding for hotel %s: %w", hotel.ID, err)
		}
	}

	return GenerateAndStoreResult{Processed: len(input.Hotels)}, nil
}

// UpdateProgressInput is the input for UpdateProgress.
type UpdateProgressInput struct {
	Version          string
	ProcessedRecords int
}

// UpdateProgress updates the processed count in the version record.
func (a *Activities) UpdateProgress(ctx context.Context, input UpdateProgressInput) error {
	return db.UpdateVersionProgress(ctx, a.Pool, input.Version, input.ProcessedRecords)
}

// ValidateMigrationInput is the input for ValidateMigration.
type ValidateMigrationInput struct {
	Version      string
	TotalRecords int
}

// ValidateMigration checks that the right number of embeddings were created.
func (a *Activities) ValidateMigration(ctx context.Context, input ValidateMigrationInput) error {
	embs, err := db.GetEmbeddingsByVersion(ctx, a.Pool, input.Version)
	if err != nil {
		return fmt.Errorf("getting embeddings for validation: %w", err)
	}

	if len(embs) != input.TotalRecords {
		return fmt.Errorf("validation failed: expected %d embeddings, got %d", input.TotalRecords, len(embs))
	}

	slog.Info("migration validated", "version", input.Version, "count", len(embs))
	return nil
}

// SwitchActiveVersionInput is the input for SwitchActiveVersion.
type SwitchActiveVersionInput struct {
	Version string
}

// SwitchActiveVersion marks the migration as completed and switches the active version.
func (a *Activities) SwitchActiveVersion(ctx context.Context, input SwitchActiveVersionInput) error {
	if err := db.CompleteVersion(ctx, a.Pool, input.Version); err != nil {
		return fmt.Errorf("completing version: %w", err)
	}

	if err := db.SetActiveVersion(ctx, a.Pool, input.Version); err != nil {
		return fmt.Errorf("setting active version: %w", err)
	}

	slog.Info("switched active version", "version", input.Version)
	return nil
}
