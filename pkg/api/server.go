package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/testsabirweb/connect_llm/internal/config"
	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/ingestion"
	"github.com/testsabirweb/connect_llm/pkg/processing"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// documentProcessorAdapter adapts processing.DocumentProcessor to ingestion.DocumentProcessor interface
type documentProcessorAdapter struct {
	processor *processing.DocumentProcessor
}

// ProcessMessage implements the ingestion.DocumentProcessor interface
func (a *documentProcessorAdapter) ProcessMessage(ctx context.Context, msg ingestion.SlackMessage) ([]vector.Document, error) {
	return a.processor.ProcessMessage(ctx, msg)
}

// Server represents the API server
type Server struct {
	config           *config.Config
	vectorClient     vector.Client
	ingestionService *ingestion.Service
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config) (*Server, error) {
	// Create Weaviate client
	vectorClient, err := vector.NewWeaviateClient(
		cfg.Weaviate.Scheme,
		cfg.Weaviate.Host,
		cfg.Weaviate.APIKey,
	)
	if err != nil {
		return nil, err
	}

	// Initialize Weaviate schema
	ctx := context.Background()
	if err := vectorClient.Initialize(ctx); err != nil {
		return nil, err
	}

	log.Println("Weaviate schema initialized successfully")

	// Create embedder and document processor
	embedder := embeddings.NewOllamaEmbedder(cfg.Ollama.URL, "llama3:8b")
	processor := processing.NewDocumentProcessor(embedder, 500, 50)

	// Wrap processor with adapter
	adapter := &documentProcessorAdapter{processor: processor}

	// Create ingestion service
	ingestionConfig := ingestion.ServiceConfig{
		BatchSize:        100,
		MaxConcurrency:   5,
		SkipEmptyContent: true,
	}
	ingestionService := ingestion.NewService(vectorClient, adapter, ingestionConfig)

	return &Server{
		config:           cfg,
		vectorClient:     vectorClient,
		ingestionService: ingestionService,
	}, nil
}

// Router returns the HTTP handler for the server
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)

	// API endpoints
	mux.HandleFunc("/api/v1/search", s.handleSearch)
	mux.HandleFunc("/api/v1/ingest", s.handleIngest)

	// Add middleware
	return s.withMiddleware(mux)
}

// withMiddleware wraps the handler with common middleware
func (s *Server) withMiddleware(h http.Handler) http.Handler {
	// Add CORS headers
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}

// handleHealth returns the health status of the server
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check Weaviate connection
	weaviateHealthy := true
	var weaviateError string
	if err := s.vectorClient.HealthCheck(ctx); err != nil {
		weaviateHealthy = false
		weaviateError = err.Error()
	}

	response := map[string]interface{}{
		"status":  "healthy",
		"service": "connect-llm",
		"checks": map[string]interface{}{
			"weaviate": map[string]interface{}{
				"healthy": weaviateHealthy,
				"error":   weaviateError,
			},
		},
	}

	// Set overall status based on component health
	if !weaviateHealthy {
		response["status"] = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSearch handles search queries
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement search functionality
	response := map[string]string{
		"message": "Search endpoint - not implemented yet",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleIngest handles data ingestion requests
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req ingestion.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Type != "file" && req.Type != "directory" {
		http.Error(w, "Invalid type: must be 'file' or 'directory'", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	// Perform ingestion based on type
	ctx := r.Context()
	var stats *ingestion.IngestionStats
	var err error

	switch req.Type {
	case "file":
		log.Printf("Starting file ingestion: %s", req.Path)
		stats, err = s.ingestionService.IngestFile(ctx, req.Path)
	case "directory":
		log.Printf("Starting directory ingestion: %s", req.Path)
		stats, err = s.ingestionService.IngestDirectory(ctx, req.Path)
	}

	// Prepare response
	var response ingestion.IngestResponse
	if err != nil {
		log.Printf("Ingestion error: %v", err)
		response = ingestion.IngestResponse{
			Success: false,
			Stats:   make(map[string]interface{}),
			Errors:  []string{err.Error()},
		}
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		// Convert errors to string array
		errorStrings := make([]string, 0, len(stats.Errors))
		for _, e := range stats.Errors {
			errorStrings = append(errorStrings, e.Error())
		}

		response = ingestion.IngestResponse{
			Success: true,
			Stats:   stats.GetSummary(),
			Errors:  errorStrings,
		}

		log.Printf("Ingestion completed successfully. Stats: %+v", stats.GetSummary())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
