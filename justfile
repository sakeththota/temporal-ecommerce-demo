# =============================================================================
# Temporal Ecommerce Demo
# =============================================================================

@_:
  just --list

# Initial setup: create .env from example and start containers
setup:
    @test -f .env || (cp .env.example .env && echo "Created .env from .env.example — edit it with your values")
    just up

# -----------------------------------------------------------------------------
# Docker
# -----------------------------------------------------------------------------

# Build and start all containers
up:
    docker compose up --build -d --remove-orphans

# Start without rebuilding
dev:
    docker compose up -d --remove-orphans

# Stop containers
down:
    docker compose down

# Stop and remove volumes (full wipe)
clean:
    docker compose down -v

# Full reset: wipe volumes and rebuild
reset:
    just clean up

# Rebuild a single service (e.g. just rebuild frontend)
rebuild SERVICE:
    docker compose up --build -d --no-deps {{SERVICE}}

# Show container status
ps:
    @docker compose ps

# View logs (e.g. just logs search)
logs SERVICE="":
    docker compose logs -f {{SERVICE}}

# -----------------------------------------------------------------------------
# Database
# -----------------------------------------------------------------------------

# Show record counts
db-status:
    @docker compose exec postgresql psql -U postgres -d hotel_booking -c \
        "SELECT 'hotels' AS table_name, COUNT(*) FROM hotels \
         UNION ALL SELECT 'hotel_embeddings', COUNT(*) FROM hotel_embeddings \
         UNION ALL SELECT 'embedding_versions', COUNT(*) FROM embedding_versions \
         UNION ALL SELECT 'bookings', COUNT(*) FROM bookings \
         UNION ALL SELECT 'booking_items', COUNT(*) FROM booking_items \
         ORDER BY table_name;"

# Open a psql shell
db-shell:
    docker compose exec postgresql psql -U postgres -d hotel_booking

# Run pending migrations
db-migrate:
    docker compose exec search goose -dir=/app/migrations up

# Reset embeddings/versions and re-seed v1 (keeps hotels)
db-reset-embeddings:
    @echo "Resetting embeddings and re-seeding v1..."
    @curl -s -X POST http://localhost/api/migrations/reset
    @echo ""
    @echo "Done. Run 'just db-status' to verify."

# Clear all bookings
db-reset-bookings:
    docker compose exec postgresql psql -U postgres -d hotel_booking -c \
        "DELETE FROM booking_items; DELETE FROM bookings;"
    @echo "Bookings cleared."

# Wipe all data and re-seed from scratch
db-reseed:
    docker compose exec postgresql psql -U postgres -d hotel_booking -c \
        "DELETE FROM booking_items; DELETE FROM bookings; DELETE FROM hotel_embeddings; DELETE FROM embedding_versions; DELETE FROM hotels;"
    @echo "All data wiped. Restarting search to re-seed..."
    docker compose restart search
    @echo "Waiting for seed to complete..."
    @until docker compose logs search --tail 1 2>&1 | grep -q "migration service starting"; do sleep 2; done
    @echo "Done. Run 'just db-status' to verify."

# -----------------------------------------------------------------------------
# Local Dev (outside Docker)
# -----------------------------------------------------------------------------

# Build all components
build:
    go build ./...
    cd frontend && npm run build

# Run tests
test:
    go test ./...

# Run linters
lint:
    golangci-lint run
    cd frontend && npm run lint
