package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/sakeththota/durable-embedding-migration/backend/internal/db"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/embeddings"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/search"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/workflows"
)

type Server struct {
	pool     *pgxpool.Pool
	ollama   *embeddings.Client
	temporal temporalclient.Client
	mux      *http.ServeMux
}

func NewServer(pool *pgxpool.Pool, ollama *embeddings.Client, tc temporalclient.Client) *Server {
	s := &Server{
		pool:     pool,
		ollama:   ollama,
		temporal: tc,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

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
	s.mux.HandleFunc("GET /api/hotels", s.handleListHotels)
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/versions", s.handleListVersions)
	s.mux.HandleFunc("POST /api/migrations", s.handleStartMigration)
	s.mux.HandleFunc("GET /api/migrations/{version}", s.handleGetMigrationProgress)
	s.mux.HandleFunc("POST /api/migrations/{version}/pause", s.handlePauseMigration)
	s.mux.HandleFunc("POST /api/migrations/{version}/resume", s.handleResumeMigration)
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
	versions, err := db.ListVersions(ctx, s.pool)
	if err != nil {
		slog.Error("listing versions", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	var modelName string
	for _, v := range versions {
		if v.Version == activeVersion {
			modelName = v.ModelName
			break
		}
	}

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
	Version    string `json:"version"`
	ModelName  string `json:"model_name"`
	Dimensions int    `json:"dimensions"`
	BatchSize  int    `json:"batch_size"`
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

	workflowID := "migration-" + req.Version

	run, err := s.temporal.ExecuteWorkflow(r.Context(), temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflows.TaskQueue,
	}, workflows.EmbeddingMigrationWorkflow, workflows.MigrationInput{
		Version:    req.Version,
		ModelName:  req.ModelName,
		Dimensions: req.Dimensions,
		BatchSize:  req.BatchSize,
	})
	if err != nil {
		slog.Error("starting migration workflow", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to start migration"})
		return
	}

	slog.Info("migration started", "workflowID", run.GetID(), "runID", run.GetRunID())
	writeJSON(w, http.StatusAccepted, map[string]string{
		"workflow_id": run.GetID(),
		"run_id":      run.GetRunID(),
		"version":     req.Version,
	})
}

func (s *Server) handleGetMigrationProgress(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	workflowID := "migration-" + version

	resp, err := s.temporal.QueryWorkflow(r.Context(), workflowID, "", workflows.QueryProgress)
	if err != nil {
		slog.Error("querying migration progress", "error", err, "version", version)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query migration progress"})
		return
	}

	var progress workflows.MigrationProgress
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "error", err)
	}
}
