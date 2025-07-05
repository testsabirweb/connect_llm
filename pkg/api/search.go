package api

import (
	"errors"
	"time"
)

// Search errors
var (
	ErrEmptyQuery    = errors.New("search query cannot be empty")
	ErrInvalidLimit  = errors.New("limit must be between 1 and 100")
	ErrInvalidOffset = errors.New("offset cannot be negative")
)

// SearchRequest represents a search query request
type SearchRequest struct {
	// Query is the search query text
	Query string `json:"query"`

	// Limit is the maximum number of results to return (default: 10, max: 100)
	Limit int `json:"limit,omitempty"`

	// Offset for pagination (default: 0)
	Offset int `json:"offset,omitempty"`

	// Filters for metadata-based filtering
	Filters *SearchFilters `json:"filters,omitempty"`
}

// SearchFilters contains metadata filters for search
type SearchFilters struct {
	// Filter by source system
	Source string `json:"source,omitempty"`

	// Filter by author
	Author string `json:"author,omitempty"`

	// Filter by tags (any match)
	Tags []string `json:"tags,omitempty"`

	// Filter by date range
	DateFrom *time.Time `json:"dateFrom,omitempty"`
	DateTo   *time.Time `json:"dateTo,omitempty"`

	// Filter by permissions (user must have access)
	RequirePermission string `json:"requirePermission,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	// Document ID
	ID string `json:"id"`

	// Content snippet (may be truncated)
	Content string `json:"content"`

	// Relevance score (0-1, higher is more relevant)
	Score float32 `json:"score"`

	// Source information
	Source   string `json:"source"`
	SourceID string `json:"sourceId"`

	// Document metadata
	Title     string    `json:"title,omitempty"`
	Author    string    `json:"author,omitempty"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
	Tags      []string  `json:"tags,omitempty"`

	// Highlighted content with search terms emphasized
	Highlights []string `json:"highlights,omitempty"`
}

// SearchResponse represents the search API response
type SearchResponse struct {
	// Search results
	Results []SearchResult `json:"results"`

	// Total number of matching documents (for pagination)
	Total int `json:"total"`

	// Number of results returned in this response
	Count int `json:"count"`

	// Offset used for this query
	Offset int `json:"offset"`

	// Query processing time in milliseconds
	ProcessingTimeMs int64 `json:"processingTimeMs"`

	// Optional search metadata
	Metadata *SearchMetadata `json:"metadata,omitempty"`
}

// SearchMetadata contains additional information about the search
type SearchMetadata struct {
	// Actual query used (after processing/expansion)
	ProcessedQuery string `json:"processedQuery,omitempty"`

	// Number of documents searched
	DocumentsSearched int `json:"documentsSearched"`

	// Applied filters summary
	FiltersApplied map[string]interface{} `json:"filtersApplied,omitempty"`
}

// Validate validates the search request
func (r *SearchRequest) Validate() error {
	if r.Query == "" {
		return ErrEmptyQuery
	}

	// Set defaults
	if r.Limit <= 0 {
		r.Limit = 10
	} else if r.Limit > 100 {
		r.Limit = 100
	}

	if r.Offset < 0 {
		r.Offset = 0
	}

	return nil
}
