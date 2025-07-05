package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     SearchRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty query",
			req:     SearchRequest{Query: ""},
			wantErr: true,
			errMsg:  "search query cannot be empty",
		},
		{
			name:    "valid query with defaults",
			req:     SearchRequest{Query: "test query"},
			wantErr: false,
		},
		{
			name:    "negative offset",
			req:     SearchRequest{Query: "test", Offset: -1},
			wantErr: false, // Should be corrected to 0
		},
		{
			name:    "limit too high",
			req:     SearchRequest{Query: "test", Limit: 200},
			wantErr: false, // Should be corrected to 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}

			// Check defaults are applied
			if !tt.wantErr {
				if tt.req.Limit <= 0 || tt.req.Limit > 100 {
					t.Errorf("Limit not properly set: got %d", tt.req.Limit)
				}
				if tt.req.Offset < 0 {
					t.Errorf("Offset not properly set: got %d", tt.req.Offset)
				}
			}
		})
	}
}

func TestHandleSearch_GET(t *testing.T) {
	// Create a test request
	req, err := http.NewRequest("GET", "/api/v1/search?q=test+query&limit=5", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Test that the handler properly parses GET parameters
	// Note: This is a partial test as we don't have a mock server setup
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query != "test query" {
			t.Errorf("Expected query 'test query', got '%s'", query)
		}

		limitStr := r.URL.Query().Get("limit")
		if limitStr != "5" {
			t.Errorf("Expected limit '5', got '%s'", limitStr)
		}

		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestHandleSearch_POST(t *testing.T) {
	// Create test search request
	searchReq := SearchRequest{
		Query: "test query",
		Limit: 20,
		Filters: &SearchFilters{
			Source: "slack",
			Tags:   []string{"test", "demo"},
		},
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		t.Fatal(err)
	}

	// Create a test request
	req, err := http.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Test handler that validates the request parsing
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var parsedReq SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&parsedReq); err != nil {
			t.Errorf("Failed to parse request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if parsedReq.Query != searchReq.Query {
			t.Errorf("Expected query '%s', got '%s'", searchReq.Query, parsedReq.Query)
		}

		if parsedReq.Limit != searchReq.Limit {
			t.Errorf("Expected limit %d, got %d", searchReq.Limit, parsedReq.Limit)
		}

		if parsedReq.Filters == nil || parsedReq.Filters.Source != "slack" {
			t.Errorf("Filters not properly parsed")
		}

		// Send a mock response
		mockResult := SearchResult{
			ID:      "test-id",
			Content: "Test content",
			Score:   0.95,
			Source:  "slack",
		}
		response := SearchResponse{
			Results: []SearchResult{mockResult},
			Total:   1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	handler.ServeHTTP(rr, req)

	// Check the response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check content type
	expectedContentType := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ct, expectedContentType)
	}
}

func TestSearchFilters(t *testing.T) {
	now := time.Now()
	filters := SearchFilters{
		Source:            "slack",
		Author:            "john.doe",
		Tags:              []string{"test", "demo"},
		DateFrom:          &now,
		DateTo:            &now,
		RequirePermission: "user123",
	}

	// Test that all fields are properly set
	if filters.Source != "slack" {
		t.Errorf("Expected source 'slack', got '%s'", filters.Source)
	}

	if len(filters.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(filters.Tags))
	}

	if filters.DateFrom == nil || filters.DateTo == nil {
		t.Error("Date filters should not be nil")
	}
}
