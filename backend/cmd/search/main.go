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

	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/activities"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/api"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/config"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/db"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/embeddings"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/workflows"
)

func main() {
	// Handle seed subcommand
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		if err := runSeed(); err != nil {
			slog.Error("seed failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// If we're recovering from a demo crash, delay startup so the presenter
	// can show Temporal retries in the UI.
	api.WaitIfCrashRecovery()

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

	acts := &activities.Activities{Pool: pool, Ollama: ollamaClient}

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

	aw := worker.New(tc, workflows.ApprovalTaskQueue, worker.Options{})
	aw.RegisterWorkflow(workflows.ApprovalMigrationWorkflow)
	aw.RegisterActivity(acts)

	go func() {
		if err := aw.Run(worker.InterruptCh()); err != nil {
			slog.Error("approval migration worker error", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("approval migration worker started", "taskQueue", workflows.ApprovalTaskQueue)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      api.NewMigrationServer(pool, ollamaClient, tc),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("migration service starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	mw.Stop()
	aw.Stop()
	slog.Info("shutdown complete")
}
