package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/config"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/db"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/embeddings"
)

const (
	versionName = "v1"
	modelName   = "all-minilm"
	dimensions  = 384
)

type Hotel struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Location      string   `json:"location"`
	PricePerNight float64  `json:"price_per_night"`
	Amenities     []string `json:"amenities"`
	ImageURL      string   `json:"image_url"`
}

func seedHotels(ctx context.Context, pool *pgxpool.Pool) error {
	seedFile := "seed/hotels.json"
	if len(os.Args) > 2 {
		seedFile = os.Args[2]
	}

	data, err := os.ReadFile(seedFile)
	if err != nil {
		return fmt.Errorf("reading seed file: %w", err)
	}

	var hotels []Hotel
	if err := json.Unmarshal(data, &hotels); err != nil {
		return fmt.Errorf("parsing seed file: %w", err)
	}

	inserted := 0
	for _, h := range hotels {
		tag, err := pool.Exec(ctx,
			`INSERT INTO hotels (name, description, location, price_per_night, amenities, image_url)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT DO NOTHING`,
			h.Name, h.Description, h.Location, h.PricePerNight, h.Amenities, h.ImageURL,
		)
		if err != nil {
			return fmt.Errorf("inserting hotel %s: %w", h.Name, err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
		}
	}

	fmt.Printf("seeded %d hotels\n", inserted)
	return nil
}

func seedEmbeddings(ctx context.Context, pool *pgxpool.Pool, ollama *embeddings.Client) error {
	hotels, err := db.ListHotels(ctx, pool)
	if err != nil {
		return fmt.Errorf("listing hotels: %w", err)
	}
	slog.Info("loaded hotels", "count", len(hotels))

	if err := db.CreateVersion(ctx, pool, versionName, modelName, dimensions, len(hotels)); err != nil {
		return fmt.Errorf("creating version: %w", err)
	}
	slog.Info("created embedding version", "version", versionName, "model", modelName)

	start := time.Now()
	for i, hotel := range hotels {
		text := hotel.Name + " " + hotel.Description

		embedding, err := ollama.Embed(ctx, modelName, text)
		if err != nil {
			return fmt.Errorf("embedding hotel %s: %w", hotel.Name, err)
		}

		if err := db.SaveEmbedding(ctx, pool, hotel.ID, versionName, modelName, len(embedding), embedding); err != nil {
			return fmt.Errorf("saving embedding for hotel %s: %w", hotel.Name, err)
		}

		if err := db.UpdateVersionProgress(ctx, pool, versionName, i+1); err != nil {
			return fmt.Errorf("updating progress: %w", err)
		}

		slog.Info("embedded hotel", "progress", fmt.Sprintf("%d/%d", i+1, len(hotels)), "hotel", hotel.Name)
	}

	if err := db.CompleteVersion(ctx, pool, versionName); err != nil {
		return fmt.Errorf("completing version: %w", err)
	}

	if err := db.SetActiveVersion(ctx, pool, versionName); err != nil {
		return fmt.Errorf("setting active version: %w", err)
	}

	elapsed := time.Since(start)
	slog.Info("seeding complete", "version", versionName, "hotels", len(hotels), "elapsed", elapsed.Round(time.Millisecond))
	return nil
}

func runSeed() error {
	ctx := context.Background()
	cfg := config.Load()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	ollamaClient := embeddings.NewClient(cfg.OllamaURL)

	existingVersion, err := db.GetVersionByName(ctx, pool, versionName)
	if err != nil {
		return fmt.Errorf("checking existing version: %w", err)
	}

	if existingVersion != nil && existingVersion.Status == "completed" {
		fmt.Printf("version %s already exists and is completed, skipping seed\n", versionName)
		return nil
	}

	if err := seedHotels(ctx, pool); err != nil {
		return err
	}

	if err := seedEmbeddings(ctx, pool, ollamaClient); err != nil {
		return err
	}

	return nil
}
