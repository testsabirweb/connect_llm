package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/testsabirweb/connect_llm/internal/config"
	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/ingestion"
	"github.com/testsabirweb/connect_llm/pkg/processing"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// This example demonstrates how to use the ingestion service programmatically

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Weaviate client
	fmt.Println("Setting up Weaviate client...")
	vectorClient, err := vector.NewWeaviateClient(
		cfg.Weaviate.Scheme,
		cfg.Weaviate.Host,
		cfg.Weaviate.APIKey,
	)
	if err != nil {
		log.Fatalf("Failed to create Weaviate client: %v", err)
	}

	// Initialize Weaviate schema
	ctx := context.Background()
	if err := vectorClient.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize Weaviate schema: %v", err)
	}

	// Create embedder with custom configuration
	embedder := embeddings.NewOllamaEmbedder(cfg.Ollama.URL, "llama3:8b")

	// Create document processor with custom chunk settings
	processor := processing.NewDocumentProcessor(
		embedder,
		500, // chunk size in words
		50,  // chunk overlap in words
	)

	// Create ingestion service with custom configuration
	service := ingestion.NewService(
		vectorClient,
		&documentProcessorAdapter{processor: processor},
		ingestion.ServiceConfig{
			BatchSize:        100,  // Process 100 messages at a time
			MaxConcurrency:   5,    // Use 5 concurrent workers
			SkipEmptyContent: true, // Skip messages with no content
		},
	)

	// Example 1: Ingest a single file
	fmt.Println("\nExample 1: Ingesting a single file...")
	stats1, err := service.IngestFile(ctx, "slack/test_messages.csv")
	if err != nil {
		log.Printf("Failed to ingest file: %v", err)
	} else {
		printStats("Single file ingestion", stats1)
	}

	// Example 2: Ingest all files in a directory
	fmt.Println("\nExample 2: Ingesting all files in a directory...")
	stats2, err := service.IngestDirectory(ctx, "slack/")
	if err != nil {
		log.Printf("Failed to ingest directory: %v", err)
	} else {
		printStats("Directory ingestion", stats2)
	}

	// Example 3: Custom batch processing
	fmt.Println("\nExample 3: Custom batch processing...")

	// Create a custom parser with specific settings
	parser := ingestion.NewCSVParser(ingestion.ParserConfig{
		BatchSize:       50,   // Smaller batches
		SkipErrors:      true, // Continue on errors
		ValidateRecords: true, // Validate each record
	})

	// Parse messages manually
	file, err := os.Open("slack/test_messages.csv")
	if err != nil {
		log.Printf("Failed to open file: %v", err)
	} else {
		defer file.Close()

		messages, err := parser.Parse(file)
		if err != nil {
			log.Printf("Failed to parse messages: %v", err)
		} else {
			fmt.Printf("Parsed %d messages\n", len(messages))

			// Process messages in custom batches
			batchSize := 25
			for i := 0; i < len(messages); i += batchSize {
				end := i + batchSize
				if end > len(messages) {
					end = len(messages)
				}

				batch := messages[i:end]
				fmt.Printf("Processing batch %d-%d...\n", i, end)

				// Process and store each message
				for _, msg := range batch {
					docs, err := processor.ProcessMessage(ctx, msg)
					if err != nil {
						log.Printf("Failed to process message %s: %v", msg.MessageID, err)
						continue
					}

					for _, doc := range docs {
						if err := vectorClient.Store(ctx, doc); err != nil {
							log.Printf("Failed to store document %s: %v", doc.ID, err)
						}
					}
				}
			}
		}
	}

	fmt.Println("\nIngestion examples completed!")
}

// documentProcessorAdapter adapts processing.DocumentProcessor to ingestion.DocumentProcessor interface
type documentProcessorAdapter struct {
	processor *processing.DocumentProcessor
}

func (a *documentProcessorAdapter) ProcessMessage(ctx context.Context, msg ingestion.SlackMessage) ([]vector.Document, error) {
	return a.processor.ProcessMessage(ctx, msg)
}

// printStats prints ingestion statistics in a formatted way
func printStats(title string, stats *ingestion.IngestionStats) {
	summary := stats.GetSummary()

	fmt.Printf("\n=== %s ===\n", title)
	fmt.Printf("Total messages: %v\n", summary["total_messages"])
	fmt.Printf("Processed: %v\n", summary["processed_messages"])
	fmt.Printf("Skipped: %v\n", summary["skipped_messages"])
	fmt.Printf("Failed: %v\n", summary["failed_messages"])
	fmt.Printf("Documents created: %v\n", summary["total_documents"])
	fmt.Printf("Documents stored: %v\n", summary["stored_documents"])
	fmt.Printf("Duration: %.2f seconds\n", summary["duration_seconds"])
	fmt.Printf("Rate: %.2f messages/second\n", summary["messages_per_second"])

	if errorCount := summary["error_count"].(int); errorCount > 0 {
		fmt.Printf("Errors: %d\n", errorCount)
		// Show first 5 errors
		for i, err := range stats.Errors {
			if i >= 5 {
				fmt.Printf("  ... and %d more errors\n", errorCount-5)
				break
			}
			fmt.Printf("  - %v\n", err)
		}
	}
}
