package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/testsabirweb/connect_llm/internal/config"
	"github.com/testsabirweb/connect_llm/pkg/chat"
	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/ingestion"
	"github.com/testsabirweb/connect_llm/pkg/models"
	"github.com/testsabirweb/connect_llm/pkg/processing"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// documentProcessorAdapter adapts processing.DocumentProcessor to ingestion.DocumentProcessor interface
type documentProcessorAdapter struct {
	processor *processing.DocumentProcessor
}

// ProcessMessage implements the ingestion.DocumentProcessor interface
func (a *documentProcessorAdapter) ProcessMessage(ctx context.Context, msg models.SlackMessage) ([]vector.Document, error) {
	return a.processor.ProcessMessage(ctx, msg)
}

// Server represents the API server
type Server struct {
	config           *config.Config
	vectorClient     vector.Client
	ingestionService *ingestion.Service
	chatHub          *chat.Hub
	chatService      *chat.Service
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

	// Create chat hub and service
	chatHub := chat.NewHub()
	chatConfig := chat.DefaultServiceConfig()
	chatConfig.OllamaURL = cfg.Ollama.URL
	chatService := chat.NewService(chatHub, vectorClient, chatConfig)

	// Start the chat hub
	go chatHub.Run(context.Background())

	return &Server{
		config:           cfg,
		vectorClient:     vectorClient,
		ingestionService: ingestionService,
		chatHub:          chatHub,
		chatService:      chatService,
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

	// Chat endpoints
	mux.HandleFunc("/api/v1/chat/ws", s.handleWebSocket)
	mux.HandleFunc("/api/v1/chat/conversations", s.handleConversations)
	mux.HandleFunc("/api/v1/chat/conversations/", s.handleConversation)

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
	json.NewEncoder(w).Encode(response) //nolint:errcheck // Response write errors are handled by HTTP framework
}

// handleSearch handles search queries
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	// Only accept GET and POST requests
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	startTime := time.Now()

	// Parse search request
	var req SearchRequest
	if r.Method == http.MethodPost {
		// Parse from request body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		// Parse from query parameters
		req.Query = r.URL.Query().Get("q")
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				req.Limit = limit
			}
		}
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			if offset, err := strconv.Atoi(offsetStr); err == nil {
				req.Offset = offset
			}
		}
	}

	// Validate request
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate embeddings for the query
	embedder := embeddings.NewOllamaEmbedder(s.config.Ollama.URL, "llama3:8b")
	queryEmbeddings, err := embedder.GenerateEmbedding(ctx, req.Query)
	if err != nil {
		log.Printf("Failed to generate embeddings: %v", err)
		http.Error(w, "Failed to process search query", http.StatusInternalServerError)
		return
	}

	// Convert filters to map if present
	var filters map[string]interface{}
	if req.Filters != nil {
		filters = make(map[string]interface{})
		if req.Filters.Source != "" {
			filters["source"] = req.Filters.Source
		}
		if req.Filters.Author != "" {
			filters["author"] = req.Filters.Author
		}
		if len(req.Filters.Tags) > 0 {
			filters["tags"] = req.Filters.Tags
		}
		if req.Filters.DateFrom != nil {
			filters["dateFrom"] = req.Filters.DateFrom.Format(time.RFC3339)
		}
		if req.Filters.DateTo != nil {
			filters["dateTo"] = req.Filters.DateTo.Format(time.RFC3339)
		}
		if req.Filters.RequirePermission != "" {
			filters["permissions"] = req.Filters.RequirePermission
		}
	}

	// Perform vector search
	searchOpts := vector.SearchOptions{
		Query:   queryEmbeddings,
		Limit:   req.Limit,
		Offset:  req.Offset,
		Filters: filters,
	}

	documents, err := s.vectorClient.SearchWithOptions(ctx, searchOpts)
	if err != nil {
		log.Printf("Search failed: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Convert documents to search results
	results := make([]SearchResult, 0, len(documents))
	for i, doc := range documents {
		// Calculate score based on position (closer = higher score)
		// In a real implementation, we would use the distance from Weaviate
		score := float32(1.0 - (float64(i) / float64(req.Limit)))

		// Truncate content for snippet
		contentSnippet := doc.Content
		if len(contentSnippet) > 500 {
			contentSnippet = contentSnippet[:497] + "..."
		}

		result := SearchResult{
			ID:        doc.ID,
			Content:   contentSnippet,
			Score:     score,
			Source:    doc.Source,
			SourceID:  doc.SourceID,
			Title:     doc.Metadata.Title,
			Author:    doc.Metadata.Author,
			URL:       doc.Metadata.URL,
			CreatedAt: doc.Metadata.CreatedAt,
			UpdatedAt: doc.Metadata.UpdatedAt,
			Tags:      doc.Metadata.Tags,
		}

		results = append(results, result)
	}

	// Calculate processing time
	processingTime := time.Since(startTime).Milliseconds()

	// Build response
	response := SearchResponse{
		Results:          results,
		Total:            len(documents), // In a real implementation, we'd get the total count from Weaviate
		Count:            len(results),
		Offset:           req.Offset,
		ProcessingTimeMs: processingTime,
		Metadata: &SearchMetadata{
			ProcessedQuery:    req.Query,
			DocumentsSearched: -1, // Unknown without full count query
			FiltersApplied:    filters,
		},
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
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
	json.NewEncoder(w).Encode(response) //nolint:errcheck // Response write errors are handled by HTTP framework
}
