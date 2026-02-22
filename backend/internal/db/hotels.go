package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Hotel struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Location      string    `json:"location"`
	PricePerNight float64   `json:"price_per_night"`
	Amenities     []string  `json:"amenities"`
	CreatedAt     time.Time `json:"created_at"`
}

func ListHotels(ctx context.Context, pool *pgxpool.Pool) ([]Hotel, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, description, location, price_per_night, amenities, created_at
		 FROM hotels
		 ORDER BY name`)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[Hotel])
}
