package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/testsabirweb/connect_llm/internal/config"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// Server represents the API server
type Server struct {
	config       *config.Config
	vectorClient vector.Client
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

	return &Server{
		config:       cfg,
		vectorClient: vectorClient,
	}, nil
}

// Router returns the HTTP handler for the server
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)

	// API endpoints will be added here
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
	// TODO: Implement ingestion functionality
	response := map[string]string{
		"message": "Ingest endpoint - not implemented yet",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
