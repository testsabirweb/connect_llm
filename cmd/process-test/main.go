package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/testsabirweb/connect_llm/internal/config"
	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/ingestion"
	"github.com/testsabirweb/connect_llm/pkg/processing"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

func main() {
	var (
		csvFile         = flag.String("file", "slack/messages.csv", "Path to CSV file")
		limit           = flag.Int("limit", 5, "Number of messages to process")
		embeddingModel  = flag.String("model", "nomic-embed-text", "Ollama model for embeddings")
		storeInWeaviate = flag.Bool("store", false, "Store documents in Weaviate")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create embedder
	embedder := embeddings.NewOllamaEmbedder(cfg.Ollama.URL, *embeddingModel)

	// Test embedding generation
	ctx := context.Background()
	fmt.Println("Testing embedding generation...")
	testEmbedding, err := embedder.GenerateEmbedding(ctx, "Hello, world!")
	if err != nil {
		log.Printf("Warning: Failed to generate test embedding: %v", err)
		log.Printf("Make sure Ollama is running and %s model is available", *embeddingModel)
		log.Printf("Run: ollama pull %s", *embeddingModel)
	} else {
		fmt.Printf("✓ Embedding generation working (dimension: %d)\n", len(testEmbedding))
	}

	// Create document processor
	chunkConfig := processing.DefaultChunkingConfig()
	processor := processing.NewDocumentProcessor(embedder, chunkConfig.MaxChunkSize, chunkConfig.ChunkOverlap)

	// Parse CSV file
	fmt.Printf("\nParsing CSV file: %s\n", *csvFile)
	parser := ingestion.NewCSVParser()

	var messages []ingestion.SlackMessage
	err = parser.ParseFile(*csvFile,
		func(batch []ingestion.SlackMessage, batchNum int) error {
			// Only process up to limit
			remaining := *limit - len(messages)
			if remaining <= 0 {
				return nil
			}

			if len(batch) > remaining {
				messages = append(messages, batch[:remaining]...)
			} else {
				messages = append(messages, batch...)
			}
			return nil
		},
		func(processed, total, errors int) {
			if processed%100 == 0 {
				fmt.Printf("\rProgress: %d/%d messages, %d errors", processed, total, errors)
			}
		},
	)
	fmt.Println() // New line after progress

	if err != nil {
		log.Fatalf("Error parsing file: %v", err)
	}

	fmt.Printf("\nProcessing %d messages...\n", len(messages))

	// Process messages
	for i, msg := range messages {
		fmt.Printf("\n--- Message %d ---\n", i+1)
		fmt.Printf("ID: %s\n", msg.MessageID)
		fmt.Printf("User: %s\n", msg.User)
		fmt.Printf("Channel: %s\n", msg.Channel)

		// Truncate content for display
		content := msg.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Printf("Content: %s\n", content)

		// Process into documents
		docs, err := processor.ProcessMessage(ctx, msg)
		if err != nil {
			log.Printf("Error processing message %s: %v", msg.MessageID, err)
			continue
		}

		fmt.Printf("Generated %d document(s)\n", len(docs))

		for j, doc := range docs {
			fmt.Printf("  Document %d:\n", j+1)
			fmt.Printf("    ID: %s\n", doc.ID)
			fmt.Printf("    Title: %s\n", doc.Metadata.Title)
			fmt.Printf("    Tags: %v\n", doc.Metadata.Tags)
			fmt.Printf("    Embedding dimension: %d\n", len(doc.Embedding))

			// Optionally store in Weaviate
			if *storeInWeaviate {
				client, err := vector.NewWeaviateClient(
					cfg.Weaviate.Scheme,
					cfg.Weaviate.Host,
					cfg.Weaviate.APIKey,
				)
				if err != nil {
					log.Printf("Failed to create Weaviate client: %v", err)
					continue
				}

				err = client.Store(ctx, doc)
				if err != nil {
					log.Printf("Failed to store document: %v", err)
				} else {
					fmt.Printf("    ✓ Stored in Weaviate\n")
				}
			}
		}
	}

	// Print summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Messages processed: %d\n", len(messages))

	// Print CSV parsing stats
	total, processed, errorCount := parser.GetStats()
	fmt.Printf("CSV stats: %d total, %d processed, %d errors\n", total, processed, errorCount)

	// Check if we can connect to services
	if *storeInWeaviate {
		client, err := vector.NewWeaviateClient(
			cfg.Weaviate.Scheme,
			cfg.Weaviate.Host,
			cfg.Weaviate.APIKey,
		)
		if err == nil {
			err = client.HealthCheck(ctx)
			if err == nil {
				fmt.Println("✓ Weaviate connection successful")
			} else {
				fmt.Printf("✗ Weaviate health check failed: %v\n", err)
			}
		}
	}

	// Display any parsing errors
	if errorCount > 0 {
		errors := parser.GetErrors()
		fmt.Printf("\n=== First 5 Parsing Errors ===\n")
		for i, err := range errors {
			if i >= 5 {
				break
			}
			fmt.Printf("%d. %v\n", i+1, err)
		}
	}
}
