-- +goose Up
ALTER TABLE hotels ADD COLUMN image_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE hotels DROP COLUMN IF EXISTS image_url;
