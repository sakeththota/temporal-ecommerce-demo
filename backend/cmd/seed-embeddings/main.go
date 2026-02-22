package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/sakeththota/durable-embedding-migration/backend/internal/config"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/db"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/embeddings"
)

const (
	versionName = "v1"
	modelName   = "all-minilm"
	dimensions  = 384
)

func main() {
	ctx := context.Background()

	cfg := config.Load()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	ollamaClient := embeddings.NewClient(cfg.OllamaURL)

	// Load all hotels.
	hotels, err := db.ListHotels(ctx, pool)
	if err != nil {
		slog.Error("failed to list hotels", "error", err)
		os.Exit(1)
	}
	slog.Info("loaded hotels", "count", len(hotels))

	// Create the v1 version record.
	if err := db.CreateVersion(ctx, pool, versionName, modelName, dimensions, len(hotels)); err != nil {
		slog.Error("failed to create version", "error", err)
		os.Exit(1)
	}
	slog.Info("created embedding version", "version", versionName, "model", modelName)

	// Embed each hotel and save.
	start := time.Now()
	for i, hotel := range hotels {
		text := hotel.Name + " " + hotel.Description

		embedding, err := ollamaClient.Embed(ctx, modelName, text)
		if err != nil {
			slog.Error("failed to embed hotel", "hotel", hotel.Name, "error", err)
			os.Exit(1)
		}

		if err := db.SaveEmbedding(ctx, pool, hotel.ID, versionName, modelName, len(embedding), embedding); err != nil {
			slog.Error("failed to save embedding", "hotel", hotel.Name, "error", err)
			os.Exit(1)
		}

		if err := db.UpdateVersionProgress(ctx, pool, versionName, i+1); err != nil {
			slog.Error("failed to update progress", "error", err)
			os.Exit(1)
		}

		slog.Info("embedded hotel", "progress", fmt.Sprintf("%d/%d", i+1, len(hotels)), "hotel", hotel.Name)
	}

	// Mark version as completed and active.
	if err := db.CompleteVersion(ctx, pool, versionName); err != nil {
		slog.Error("failed to complete version", "error", err)
		os.Exit(1)
	}

	if err := db.SetActiveVersion(ctx, pool, versionName); err != nil {
		slog.Error("failed to set active version", "error", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	slog.Info("seeding complete", "version", versionName, "hotels", len(hotels), "elapsed", elapsed.Round(time.Millisecond))
}
