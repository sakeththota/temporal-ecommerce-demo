package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/sakeththota/durable-embedding-migration/backend/internal/activities"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/api"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/config"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/db"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/embeddings"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/workflows"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	ollamaClient := embeddings.NewClient(cfg.OllamaURL)

	// Connect to Temporal with retry logic for restart resilience.
	var tc temporalclient.Client
	for attempt := range 10 {
		tc, err = temporalclient.Dial(temporalclient.Options{
			HostPort: cfg.TemporalAddress,
		})
		if err == nil {
			slog.Info("connected to temporal")
			break
		}
		slog.Warn("failed to connect to temporal, retrying",
			"attempt", attempt+1,
			"error", err,
		)
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		slog.Error("failed to connect to temporal after retries", "error", err)
		os.Exit(1)
	}
	defer tc.Close()

	// Start Temporal worker.
	acts := &activities.Activities{Pool: pool, Ollama: ollamaClient}

	// Migration workflow worker
	mw := worker.New(tc, workflows.TaskQueue, worker.Options{})
	mw.RegisterWorkflow(workflows.EmbeddingMigrationWorkflow)
	mw.RegisterActivity(acts)

	go func() {
		if err := mw.Run(worker.InterruptCh()); err != nil {
			slog.Error("migration worker error", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("migration worker started", "taskQueue", workflows.TaskQueue)

	// Booking workflow worker
	bw := worker.New(tc, workflows.BookingTaskQueue, worker.Options{})
	bw.RegisterWorkflow(workflows.BookingCheckoutWorkflow)
	bw.RegisterActivity(acts)

	go func() {
		if err := bw.Run(worker.InterruptCh()); err != nil {
			slog.Error("booking worker error", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("booking worker started", "taskQueue", workflows.BookingTaskQueue)

	// Start HTTP server.
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      api.NewServer(pool, ollamaClient, tc),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("api server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Block until we receive a shutdown signal.
	<-ctx.Done()
	slog.Info("shutting down")

	// Give active connections 10 seconds to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	mw.Stop()
	bw.Stop()
	slog.Info("shutdown complete")
}
