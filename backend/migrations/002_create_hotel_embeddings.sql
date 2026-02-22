-- +goose Up
CREATE TABLE hotel_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hotel_id UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    model_name TEXT NOT NULL,
    dimensions INT NOT NULL,
    embedding DOUBLE PRECISION[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (hotel_id, version)
);

CREATE INDEX idx_hotel_embeddings_version ON hotel_embeddings(version);

-- +goose Down
DROP TABLE hotel_embeddings;
