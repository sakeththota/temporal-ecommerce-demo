package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/activities"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/db"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/embeddings"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/search"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/workflows"
)

type ServerMode string

const (
	ServerModeMigration ServerMode = "migration"
	ServerModeBooking   ServerMode = "booking"
)

// Server is the HTTP API server, wiring routes to handlers.
type Server struct {
	pool     *pgxpool.Pool
	ollama   *embeddings.Client
	temporal temporalclient.Client
	mux      *http.ServeMux
	mode     ServerMode
}

// NewServer creates a Server with all routes registered.
func NewServer(pool *pgxpool.Pool, ollama *embeddings.Client, tc temporalclient.Client, mode ServerMode) *Server {
	s := &Server{
		pool:     pool,
		ollama:   ollama,
		temporal: tc,
		mux:      http.NewServeMux(),
		mode:     mode,
	}
	s.routes()
	return s
}

// NewMigrationServer creates a Server with migration-related routes only.
func NewMigrationServer(pool *pgxpool.Pool, ollama *embeddings.Client, tc temporalclient.Client) *Server {
	return NewServer(pool, ollama, tc, ServerModeMigration)
}

// NewBookingServer creates a Server with booking-related routes only.
func NewBookingServer(pool *pgxpool.Pool, ollama *embeddings.Client, tc temporalclient.Client) *Server {
	return NewServer(pool, ollama, tc, ServerModeBooking)
}

// ServeHTTP implements http.Handler with CORS support.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	switch s.mode {
	case ServerModeMigration:
		s.mux.HandleFunc("GET /api/hotels", s.handleListHotels)
		s.mux.HandleFunc("GET /api/search", s.handleSearch)
		s.mux.HandleFunc("GET /api/versions", s.handleListVersions)
		s.mux.HandleFunc("POST /api/migrations", s.handleStartMigration)
		s.mux.HandleFunc("POST /api/migrations/reset", s.handleResetMigrations)
		s.mux.HandleFunc("GET /api/migrations/{version}", s.handleGetMigrationProgress)
		s.mux.HandleFunc("POST /api/migrations/{version}/pause", s.handlePauseMigration)
		s.mux.HandleFunc("POST /api/migrations/{version}/resume", s.handleResumeMigration)
		s.mux.HandleFunc("POST /api/migrations/{version}/update", s.handleUpdateMigration)
		s.mux.HandleFunc("POST /api/migrations/{version}/approve", s.handleApproveMigration)
		s.mux.HandleFunc("POST /api/migrations/{version}/reject", s.handleRejectMigration)
		s.mux.HandleFunc("POST /api/crash", s.handleCrash)

	case ServerModeBooking:
		s.mux.HandleFunc("POST /api/bookings", s.handleCreateBooking)
		s.mux.HandleFunc("GET /api/bookings/{workflow_id}", s.handleGetBooking)
		s.mux.HandleFunc("POST /api/bookings/{workflow_id}/cancel", s.handleCancelBooking)
		s.mux.HandleFunc("GET /api/bookings", s.handleListBookings)
		s.mux.HandleFunc("POST /api/crash", s.handleCrash)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListHotels(w http.ResponseWriter, r *http.Request) {
	hotels, err := db.ListHotels(r.Context(), s.pool)
	if err != nil {
		slog.Error("listing hotels", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, hotels)
}

type searchResult struct {
	Hotel      db.Hotel `json:"hotel"`
	Similarity float64  `json:"similarity"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter 'q' is required"})
		return
	}

	ctx := r.Context()

	// Find the active embedding version.
	activeVersion, err := db.GetActiveVersion(ctx, s.pool)
	if err != nil {
		slog.Error("getting active version", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if activeVersion == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no active embedding version"})
		return
	}

	// Look up the version to get the model name.
	ver, err := db.GetVersionByName(ctx, s.pool, activeVersion)
	if err != nil {
		slog.Error("getting version details", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if ver == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "active version not found"})
		return
	}
	modelName := ver.ModelName

	// Embed the query text.
	queryEmbedding, err := s.ollama.Embed(ctx, modelName, query)
	if err != nil {
		slog.Error("embedding query", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to embed query"})
		return
	}

	// Load all embeddings for the active version.
	hotelEmbeddings, err := db.GetEmbeddingsByVersion(ctx, s.pool, activeVersion)
	if err != nil {
		slog.Error("getting embeddings", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Load all hotels for joining.
	hotels, err := db.ListHotels(ctx, s.pool)
	if err != nil {
		slog.Error("listing hotels", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	hotelMap := make(map[string]db.Hotel, len(hotels))
	for _, h := range hotels {
		hotelMap[h.ID] = h
	}

	// Compute similarities and rank.
	results := make([]searchResult, 0, len(hotelEmbeddings))
	for _, he := range hotelEmbeddings {
		sim := search.CosineSimilarity(queryEmbedding, he.Embedding)
		if hotel, ok := hotelMap[he.HotelID]; ok {
			results = append(results, searchResult{Hotel: hotel, Similarity: sim})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Return top 10.
	if len(results) > 10 {
		results = results[:10]
	}

	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := db.ListVersions(r.Context(), s.pool)
	if err != nil {
		slog.Error("listing versions", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, versions)
}

type startMigrationRequest struct {
	Version          string `json:"version"`
	ModelName        string `json:"model_name"`
	Dimensions       int    `json:"dimensions"`
	BatchSize        int    `json:"batch_size"`
	ApprovalWorkflow bool   `json:"approval_workflow"`
	ApprovalTimeout  int    `json:"approval_timeout_minutes"`
}

func (s *Server) handleStartMigration(w http.ResponseWriter, r *http.Request) {
	var req startMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Version == "" || req.ModelName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "version and model_name are required"})
		return
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 10
	}
	if req.Dimensions <= 0 {
		req.Dimensions = 768
	}
	if req.ApprovalTimeout <= 0 {
		req.ApprovalTimeout = 60
	}

	var workflowID string
	var taskQueue string
	var run interface {
		GetID() string
		GetRunID() string
	}
	var err error

	if req.ApprovalWorkflow {
		workflowID = "approval-migration-" + req.Version
		taskQueue = workflows.ApprovalTaskQueue
		run, err = s.temporal.ExecuteWorkflow(r.Context(), temporalclient.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: taskQueue,
		}, workflows.ApprovalMigrationWorkflow, workflows.ApprovalMigrationInput{
			Version:         req.Version,
			ModelName:       req.ModelName,
			Dimensions:      req.Dimensions,
			BatchSize:       req.BatchSize,
			ApprovalTimeout: time.Duration(req.ApprovalTimeout) * time.Minute,
		})
	} else {
		workflowID = "migration-" + req.Version
		taskQueue = workflows.TaskQueue
		run, err = s.temporal.ExecuteWorkflow(r.Context(), temporalclient.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: taskQueue,
		}, workflows.EmbeddingMigrationWorkflow, workflows.MigrationInput{
			Version:    req.Version,
			ModelName:  req.ModelName,
			Dimensions: req.Dimensions,
			BatchSize:  req.BatchSize,
		})
	}

	if err != nil {
		slog.Error("starting migration workflow", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to start migration"})
		return
	}

	slog.Info("migration started", "workflowID", run.GetID(), "runID", run.GetRunID(), "approvalWorkflow", req.ApprovalWorkflow)
	writeJSON(w, http.StatusAccepted, map[string]string{
		"workflow_id":       run.GetID(),
		"run_id":            run.GetRunID(),
		"version":           req.Version,
		"approval_workflow": fmt.Sprintf("%t", req.ApprovalWorkflow),
	})
}

func (s *Server) handleResetMigrations(w http.ResponseWriter, r *http.Request) {
	// Embedding all hotels takes well over the default 30s WriteTimeout.
	// Extend the deadline for this handler only using ResponseController.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Now().Add(10 * time.Minute)); err != nil {
		slog.Error("extending write deadline", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server does not support deadline extension"})
		return
	}

	// Use a detached context so the embedding work isn't tied to the request lifecycle.
	ctx := context.Background()

	if err := db.ResetMigrations(ctx, s.pool); err != nil {
		slog.Error("resetting migrations", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset migrations"})
		return
	}

	hotels, err := db.ListHotels(ctx, s.pool)
	if err != nil {
		slog.Error("listing hotels after reset", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list hotels"})
		return
	}

	const versionName = "v1"
	const modelName = "all-minilm"
	const dimensions = 384

	if err := db.CreateVersion(ctx, s.pool, versionName, modelName, dimensions, len(hotels)); err != nil {
		slog.Error("creating version after reset", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create version"})
		return
	}

	// Generate and store v1 embeddings for every hotel synchronously.
	for i, hotel := range hotels {
		text := hotel.Name + " " + hotel.Description
		embedding, err := s.ollama.Embed(ctx, modelName, text)
		if err != nil {
			slog.Error("embedding hotel after reset", "hotel", hotel.Name, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to embed hotel: " + hotel.Name})
			return
		}

		if err := db.SaveEmbedding(ctx, s.pool, hotel.ID, versionName, modelName, len(embedding), embedding); err != nil {
			slog.Error("saving embedding after reset", "hotel", hotel.Name, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save embedding"})
			return
		}

		if err := db.UpdateVersionProgress(ctx, s.pool, versionName, i+1); err != nil {
			slog.Error("updating progress after reset", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update progress"})
			return
		}
	}

	if err := db.CompleteVersion(ctx, s.pool, versionName); err != nil {
		slog.Error("completing version after reset", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to complete version"})
		return
	}

	if err := db.SetActiveVersion(ctx, s.pool, versionName); err != nil {
		slog.Error("setting active version after reset", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set active version"})
		return
	}

	slog.Info("migrations reset and v1 re-seeded", "hotels", len(hotels))
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset", "version": versionName})
}

func (s *Server) handleGetMigrationProgress(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")

	// Try regular migration first
	workflowID := "migration-" + version
	resp, err := s.temporal.QueryWorkflow(r.Context(), workflowID, "", workflows.QueryProgress)
	if err == nil {
		var progress workflows.MigrationProgress
		if err := resp.Get(&progress); err == nil {
			writeJSON(w, http.StatusOK, progress)
			return
		}
	}

	// Try approval migration
	workflowID = "approval-migration-" + version
	resp, err = s.temporal.QueryWorkflow(r.Context(), workflowID, "", workflows.QueryApprovalStatus)
	if err != nil {
		slog.Error("querying migration progress", "error", err, "version", version)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query migration progress"})
		return
	}

	var progress workflows.ApprovalMigrationProgress
	if err := resp.Get(&progress); err != nil {
		slog.Error("decoding migration progress", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to decode progress"})
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

func (s *Server) handlePauseMigration(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	workflowID := "migration-" + version

	err := s.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.SignalPause, nil)
	if err != nil {
		slog.Error("pausing migration", "error", err, "version", version)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to pause migration"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "paused", "version": version})
}

func (s *Server) handleResumeMigration(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	workflowID := "migration-" + version

	err := s.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.SignalResume, nil)
	if err != nil {
		slog.Error("resuming migration", "error", err, "version", version)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resume migration"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed", "version": version})
}

type createBookingRequest struct {
	GuestName  string `json:"guest_name"`
	GuestEmail string `json:"guest_email"`
	Items      []struct {
		HotelID       string  `json:"hotel_id"`
		CheckIn       string  `json:"check_in"`
		CheckOut      string  `json:"check_out"`
		Nights        int     `json:"nights"`
		PricePerNight float64 `json:"price_per_night"`
		Subtotal      float64 `json:"subtotal"`
	} `json:"items"`
}

func (s *Server) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.GuestName == "" || req.GuestEmail == "" || len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "guest_name, guest_email, and items are required"})
		return
	}

	workflowID := "booking-" + uuid.New().String()[:8]

	items := make([]activities.BookingItemInput, len(req.Items))
	for i, item := range req.Items {
		checkIn, err := time.Parse("2006-01-02", item.CheckIn)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid check_in date format"})
			return
		}
		checkOut, err := time.Parse("2006-01-02", item.CheckOut)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid check_out date format"})
			return
		}
		items[i] = activities.BookingItemInput{
			HotelID:       item.HotelID,
			CheckIn:       checkIn,
			CheckOut:      checkOut,
			Nights:        item.Nights,
			PricePerNight: item.PricePerNight,
			Subtotal:      item.Subtotal,
		}
	}

	run, err := s.temporal.ExecuteWorkflow(r.Context(), temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflows.BookingTaskQueue,
	}, workflows.BookingCheckoutWorkflow, workflows.BookingInput{
		WorkflowID: workflowID,
		GuestName:  req.GuestName,
		GuestEmail: req.GuestEmail,
		Items:      items,
	})
	if err != nil {
		slog.Error("starting booking workflow", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to start booking"})
		return
	}

	slog.Info("booking started", "workflowID", run.GetID())
	writeJSON(w, http.StatusAccepted, map[string]string{
		"workflow_id": run.GetID(),
	})
}

func (s *Server) handleGetBooking(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("workflow_id")

	resp, err := s.temporal.QueryWorkflow(r.Context(), workflowID, "", workflows.QueryBookingStatus)
	if err != nil {
		slog.Error("querying booking status", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query booking"})
		return
	}

	var progress workflows.BookingProgress
	if err := resp.Get(&progress); err != nil {
		slog.Error("decoding booking progress", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to decode progress"})
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

func (s *Server) handleListBookings(w http.ResponseWriter, r *http.Request) {
	bookings, err := db.ListBookings(r.Context(), s.pool)
	if err != nil {
		slog.Error("listing bookings", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, bookings)
}

func (s *Server) handleCrash(w http.ResponseWriter, r *http.Request) {
	slog.Info("crash endpoint called - terminating server in 1 second")
	go func() {
		time.Sleep(1 * time.Second)
		slog.Error("server crash triggered — exiting process", "error", "intentional crash for demo")
		os.Exit(1)
	}()
	writeJSON(w, http.StatusOK, map[string]string{"status": "crashing"})
}

type updateMigrationRequest struct {
	BatchSize int `json:"batch_size"`
}

func (s *Server) handleUpdateMigration(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	workflowID := "approval-migration-" + version

	var req updateMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	err := s.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.SignalUpdateParams, workflows.ApprovalMigrationUpdate{
		BatchSize: req.BatchSize,
	})
	if err != nil {
		slog.Error("updating migration", "error", err, "version", version)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update migration"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "version": version})
}

func (s *Server) handleApproveMigration(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	workflowID := "approval-migration-" + version

	err := s.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.SignalApprove, nil)
	if err != nil {
		slog.Error("approving migration", "error", err, "version", version)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to approve migration"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "version": version})
}

func (s *Server) handleRejectMigration(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	workflowID := "approval-migration-" + version

	err := s.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.SignalReject, nil)
	if err != nil {
		slog.Error("rejecting migration", "error", err, "version", version)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reject migration"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected", "version": version})
}

func (s *Server) handleCancelBooking(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("workflow_id")

	err := s.temporal.SignalWorkflow(r.Context(), workflowID, "", workflows.SignalCancelBooking, nil)
	if err != nil {
		slog.Error("cancelling booking", "error", err, "workflowID", workflowID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel booking"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "workflow_id": workflowID})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "error", err)
	}
}
