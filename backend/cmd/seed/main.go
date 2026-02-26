package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Hotel struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Location      string   `json:"location"`
	PricePerNight float64  `json:"price_per_night"`
	Amenities     []string `json:"amenities"`
	ImageURL      string   `json:"image_url"`
}

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/embeddings?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	seedFile := "seed/hotels.json"
	if len(os.Args) > 1 {
		seedFile = os.Args[1]
	}

	data, err := os.ReadFile(seedFile)
	if err != nil {
		log.Fatalf("failed to read seed file: %v", err)
	}

	var hotels []Hotel
	if err := json.Unmarshal(data, &hotels); err != nil {
		log.Fatalf("failed to parse seed file: %v", err)
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
			log.Fatalf("failed to insert hotel %q: %v", h.Name, err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
		}
	}

	fmt.Printf("seeded %d hotels (%d total in file)\n", inserted, len(hotels))
}
