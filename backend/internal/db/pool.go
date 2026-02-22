package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a connection pool with retry logic for startup resilience.
// This handles the common case where the database isn't ready yet (e.g., in Docker).
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	for attempt := range 5 {
		pool, err = pgxpool.New(ctx, databaseURL)
		if err != nil {
			slog.Warn("failed to create pool, retrying", "attempt", attempt+1, "error", err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		if err = pool.Ping(ctx); err != nil {
			pool.Close()
			slog.Warn("failed to ping database, retrying", "attempt", attempt+1, "error", err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		slog.Info("connected to database")
		return pool, nil
	}

	return nil, fmt.Errorf("connecting to database after 5 attempts: %w", err)
}
