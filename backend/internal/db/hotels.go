package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Hotel represents a hotel listing.
type Hotel struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Location      string    `json:"location"`
	PricePerNight float64   `json:"price_per_night"`
	Amenities     []string  `json:"amenities"`
	ImageURL      string    `json:"image_url"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListHotels returns all hotels ordered by name.
func ListHotels(ctx context.Context, pool *pgxpool.Pool) ([]Hotel, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, description, location, price_per_night, amenities, image_url, created_at
		 FROM hotels
		 ORDER BY name`)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[Hotel])
}

// CountHotels returns the total number of hotels.
func CountHotels(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM hotels`).Scan(&count)
	return count, err
}

// GetHotelBatch returns a page of hotels ordered by name for deterministic batching.
func GetHotelBatch(ctx context.Context, pool *pgxpool.Pool, offset, limit int) ([]Hotel, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, description, location, price_per_night, amenities, image_url, created_at
		 FROM hotels
		 ORDER BY name
		 OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[Hotel])
}
