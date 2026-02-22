package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HotelEmbedding struct {
	ID         string    `json:"id"`
	HotelID    string    `json:"hotel_id"`
	Version    string    `json:"version"`
	ModelName  string    `json:"model_name"`
	Dimensions int       `json:"dimensions"`
	Embedding  []float64 `json:"embedding"`
	CreatedAt  time.Time `json:"created_at"`
}

type EmbeddingVersion struct {
	Version          string     `json:"version"`
	ModelName        string     `json:"model_name"`
	Dimensions       int        `json:"dimensions"`
	Status           string     `json:"status"`
	TotalRecords     int        `json:"total_records"`
	ProcessedRecords int        `json:"processed_records"`
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at"`
}

// SaveEmbedding upserts an embedding for a hotel+version pair.
func SaveEmbedding(ctx context.Context, pool *pgxpool.Pool, hotelID, version, modelName string, dimensions int, embedding []float64) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO hotel_embeddings (hotel_id, version, model_name, dimensions, embedding)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (hotel_id, version) DO UPDATE
		 SET model_name = EXCLUDED.model_name,
		     dimensions = EXCLUDED.dimensions,
		     embedding = EXCLUDED.embedding`,
		hotelID, version, modelName, dimensions, embedding)
	if err != nil {
		return fmt.Errorf("saving embedding: %w", err)
	}
	return nil
}

// GetEmbeddingsByVersion returns all embeddings for a given version.
func GetEmbeddingsByVersion(ctx context.Context, pool *pgxpool.Pool, version string) ([]HotelEmbedding, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, hotel_id, version, model_name, dimensions, embedding, created_at
		 FROM hotel_embeddings
		 WHERE version = $1`, version)
	if err != nil {
		return nil, fmt.Errorf("querying embeddings: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[HotelEmbedding])
}

// CreateVersion inserts a new embedding version record.
func CreateVersion(ctx context.Context, pool *pgxpool.Pool, version, modelName string, dimensions, totalRecords int) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO embedding_versions (version, model_name, dimensions, status, total_records)
		 VALUES ($1, $2, $3, 'pending', $4)`,
		version, modelName, dimensions, totalRecords)
	if err != nil {
		return fmt.Errorf("creating version: %w", err)
	}
	return nil
}

// SetActiveVersion marks the given version as active and deactivates all others.
func SetActiveVersion(ctx context.Context, pool *pgxpool.Pool, version string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE embedding_versions SET is_active = FALSE WHERE is_active = TRUE`); err != nil {
		return fmt.Errorf("deactivating versions: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE embedding_versions SET is_active = TRUE WHERE version = $1`, version); err != nil {
		return fmt.Errorf("activating version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// GetActiveVersion returns the currently active version name, or empty string if none.
func GetActiveVersion(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var version string
	err := pool.QueryRow(ctx, `SELECT version FROM embedding_versions WHERE is_active = TRUE`).Scan(&version)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting active version: %w", err)
	}
	return version, nil
}

// ListVersions returns all embedding versions ordered by creation time.
func ListVersions(ctx context.Context, pool *pgxpool.Pool) ([]EmbeddingVersion, error) {
	rows, err := pool.Query(ctx,
		`SELECT version, model_name, dimensions, status, total_records, processed_records, is_active, created_at, completed_at
		 FROM embedding_versions
		 ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing versions: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[EmbeddingVersion])
}

// UpdateVersionProgress updates the processed count for a version.
func UpdateVersionProgress(ctx context.Context, pool *pgxpool.Pool, version string, processedRecords int) error {
	_, err := pool.Exec(ctx,
		`UPDATE embedding_versions SET processed_records = $1 WHERE version = $2`,
		processedRecords, version)
	if err != nil {
		return fmt.Errorf("updating version progress: %w", err)
	}
	return nil
}

// CompleteVersion marks a version as completed.
func CompleteVersion(ctx context.Context, pool *pgxpool.Pool, version string) error {
	_, err := pool.Exec(ctx,
		`UPDATE embedding_versions SET status = 'completed', completed_at = NOW() WHERE version = $1`,
		version)
	if err != nil {
		return fmt.Errorf("completing version: %w", err)
	}
	return nil
}
