FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY backend/ backend/
COPY seed/ seed/

RUN CGO_ENABLED=0 go build -o /app/bin/search ./backend/cmd/search
RUN CGO_ENABLED=0 go build -o /app/bin/booking ./backend/cmd/booking

RUN CGO_ENABLED=0 go install -tags goose github.com/pressly/goose/cmd/goose@latest

COPY backend/migrations/ /app/migrations/

FROM alpine:3.20 AS base
RUN apk --no-cache add ca-certificates
WORKDIR /app

FROM base AS search
COPY --from=builder /app/bin/search /app/bin/search
COPY --from=builder /app/migrations/ /app/migrations/
COPY --from=builder /app/seed/ /app/seed/
COPY --from=builder /go/bin/goose /usr/local/bin/goose
ENTRYPOINT ["/app/bin/search"]

FROM base AS booking
COPY --from=builder /app/bin/booking /app/bin/booking
COPY --from=builder /app/migrations/ /app/migrations/
COPY --from=builder /go/bin/goose /usr/local/bin/goose
ENTRYPOINT ["/app/bin/booking"]
