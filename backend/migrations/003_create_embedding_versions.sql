-- +goose Up
CREATE TABLE embedding_versions (
    version TEXT PRIMARY KEY,
    model_name TEXT NOT NULL,
    dimensions INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    total_records INT NOT NULL DEFAULT 0,
    processed_records INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE embedding_versions;
