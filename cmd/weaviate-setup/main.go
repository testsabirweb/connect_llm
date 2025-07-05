package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/testsabirweb/connect_llm/internal/config"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Weaviate client
	fmt.Printf("Connecting to Weaviate at %s://%s...\n", cfg.Weaviate.Scheme, cfg.Weaviate.Host)
	client, err := vector.NewWeaviateClient(
		cfg.Weaviate.Scheme,
		cfg.Weaviate.Host,
		cfg.Weaviate.APIKey,
	)
	if err != nil {
		log.Fatalf("Failed to create Weaviate client: %v", err)
	}

	ctx := context.Background()

	// Check health
	fmt.Println("Checking Weaviate health...")
	if err := client.HealthCheck(ctx); err != nil {
		log.Fatalf("Weaviate health check failed: %v", err)
	}
	fmt.Println("✓ Weaviate is healthy")

	// Initialize schema
	fmt.Println("Initializing Document schema...")
	if err := client.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}
	fmt.Println("✓ Schema initialized successfully")

	// Test document operations
	if len(os.Args) > 1 && os.Args[1] == "test" {
		fmt.Println("\nTesting document operations...")
		testDocument(ctx, client)
	}

	fmt.Println("\nWeaviate setup completed successfully!")
}

func testDocument(ctx context.Context, client vector.Client) {
	// Create a test document
	doc := vector.Document{
		ID:       "setup-test-doc",
		Content:  "This is a test document created during Weaviate setup",
		Source:   "setup-test",
		SourceID: "test-1",
		Metadata: vector.DocumentMetadata{
			Title:       "Setup Test Document",
			Author:      "Weaviate Setup Tool",
			Permissions: []string{"admin"},
			Tags:        []string{"test", "setup"},
			URL:         "https://example.com/setup-test",
		},
		// Simple test embedding
		Embedding: make([]float32, 384), // Typical embedding size
	}

	// Fill embedding with some test values
	for i := range doc.Embedding {
		doc.Embedding[i] = float32(i) / 384.0
	}

	// Store the document
	fmt.Printf("Storing test document with ID: %s...\n", doc.ID)
	if err := client.Store(ctx, doc); err != nil {
		log.Printf("Failed to store document: %v", err)
		return
	}
	fmt.Println("✓ Document stored successfully")

	// Delete the test document
	fmt.Printf("Deleting test document...\n")
	if err := client.Delete(ctx, doc.ID); err != nil {
		log.Printf("Failed to delete document: %v", err)
		return
	}
	fmt.Println("✓ Document deleted successfully")
}
