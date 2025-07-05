package vector

import (
	"context"
	"os"
	"testing"
	"time"
)

// These are integration tests that require Weaviate to be running.
// To run these tests:
//   1. Start Weaviate: make docker-up
//   2. Run tests: make test-integration
//      or: INTEGRATION_TEST=true go test -v ./pkg/vector/...

// Helper function to check if Weaviate is available
func isWeaviateAvailable() bool {
	// Check if we're explicitly running integration tests
	if os.Getenv("INTEGRATION_TEST") != "true" {
		return false
	}

	// Try to create a client and check health
	client, err := NewWeaviateClient("http", "localhost:8000", "")
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.HealthCheck(ctx)
	return err == nil
}

func TestWeaviateClient(t *testing.T) {
	// Skip if Weaviate is not available
	if !isWeaviateAvailable() {
		t.Skip("Skipping integration test: Weaviate is not available. Run with INTEGRATION_TEST=true and ensure Weaviate is running on localhost:8000")
	}

	ctx := context.Background()

	// Create client - assumes Weaviate is running on localhost:8000
	client, err := NewWeaviateClient("http", "localhost:8000", "")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test health check
	t.Run("HealthCheck", func(t *testing.T) {
		err := client.HealthCheck(ctx)
		if err != nil {
			t.Errorf("Health check failed: %v", err)
		}
	})

	// Test schema initialization
	t.Run("Initialize", func(t *testing.T) {
		err := client.Initialize(ctx)
		if err != nil {
			t.Errorf("Failed to initialize schema: %v", err)
		}

		// Run again to test idempotency
		err = client.Initialize(ctx)
		if err != nil {
			t.Errorf("Second initialization failed (should be idempotent): %v", err)
		}
	})

	// Test document CRUD operations
	t.Run("DocumentCRUD", func(t *testing.T) {
		// Create a test document
		doc := Document{
			ID:       "test-doc-1",
			Content:  "This is a test document for Weaviate integration",
			Source:   "test",
			SourceID: "test-1",
			Metadata: DocumentMetadata{
				Title:       "Test Document",
				Author:      "Test Author",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				Permissions: []string{"user1", "user2"},
				Tags:        []string{"test", "integration"},
				URL:         "https://example.com/test",
			},
			// For testing, we'll use a simple embedding
			Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		}

		// Store the document
		err := client.Store(ctx, doc)
		if err != nil {
			t.Errorf("Failed to store document: %v", err)
		}

		// Search for similar documents
		results, err := client.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4, 0.5}, 10)
		if err != nil {
			t.Logf("Search not fully implemented yet: %v", err)
		} else if len(results) == 0 {
			t.Log("No search results returned")
		}

		// Delete the document
		err = client.Delete(ctx, doc.ID)
		if err != nil {
			t.Errorf("Failed to delete document: %v", err)
		}
	})
}

func TestDocumentMetadata(t *testing.T) {
	// Test that metadata structure works correctly
	now := time.Now()
	meta := DocumentMetadata{
		Title:       "Test Title",
		Author:      "Test Author",
		CreatedAt:   now,
		UpdatedAt:   now,
		Permissions: []string{"user1", "user2"},
		Tags:        []string{"tag1", "tag2"},
		URL:         "https://example.com",
	}

	// Basic validation
	if meta.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", meta.Title)
	}
	if len(meta.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(meta.Permissions))
	}
	if len(meta.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(meta.Tags))
	}
}
