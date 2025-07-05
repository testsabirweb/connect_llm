package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/testsabirweb/connect_llm/internal/config"
	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/ingestion"
	"github.com/testsabirweb/connect_llm/pkg/processing"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

func main() {
	// Define command-line flags
	var (
		inputPath      = flag.String("input", "", "Path to CSV file or directory to ingest (required)")
		inputType      = flag.String("type", "auto", "Input type: 'file', 'directory', or 'auto' (default: auto)")
		batchSize      = flag.Int("batch-size", 100, "Number of messages to process in each batch")
		maxConcurrency = flag.Int("concurrency", 5, "Maximum number of concurrent workers")
		chunkSize      = flag.Int("chunk-size", 500, "Maximum chunk size in words")
		chunkOverlap   = flag.Int("chunk-overlap", 50, "Chunk overlap in words")
		skipEmpty      = flag.Bool("skip-empty", true, "Skip messages with empty content")
		embeddingModel = flag.String("embedding-model", "llama3:8b", "Ollama model to use for embeddings")
		help           = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	if *help || *inputPath == "" {
		printUsage()
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Weaviate client
	log.Println("Connecting to Weaviate...")
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

	// Create embedder and document processor
	log.Printf("Creating embedder with model: %s", *embeddingModel)
	embedder := embeddings.NewOllamaEmbedder(cfg.Ollama.URL, *embeddingModel)
	processor := processing.NewDocumentProcessor(embedder, *chunkSize, *chunkOverlap)

	// Create ingestion service
	ingestionConfig := ingestion.ServiceConfig{
		BatchSize:        *batchSize,
		MaxConcurrency:   *maxConcurrency,
		SkipEmptyContent: *skipEmpty,
	}

	// Create adapter for processor
	adapter := &documentProcessorAdapter{processor: processor}
	service := ingestion.NewService(vectorClient, adapter, ingestionConfig)

	// Determine input type
	if *inputType == "auto" {
		fileInfo, err := os.Stat(*inputPath)
		if err != nil {
			log.Fatalf("Failed to stat input path: %v", err)
		}
		if fileInfo.IsDir() {
			*inputType = "directory"
		} else {
			*inputType = "file"
		}
	}

	// Perform ingestion
	startTime := time.Now()
	var stats *ingestion.IngestionStats

	switch *inputType {
	case "file":
		log.Printf("Ingesting file: %s", *inputPath)
		stats, err = service.IngestFile(ctx, *inputPath)
	case "directory":
		log.Printf("Ingesting directory: %s", *inputPath)
		stats, err = service.IngestDirectory(ctx, *inputPath)
	default:
		log.Fatalf("Invalid input type: %s", *inputType)
	}

	if err != nil {
		log.Fatalf("Ingestion failed: %v", err)
	}

	// Print results
	duration := time.Since(startTime)
	fmt.Println("\n=== Ingestion Complete ===")
	fmt.Printf("Duration: %s\n", duration.Round(time.Second))
	fmt.Printf("Total messages: %d\n", stats.TotalMessages)
	fmt.Printf("Processed messages: %d\n", stats.ProcessedMessages)
	fmt.Printf("Skipped messages: %d\n", stats.SkippedMessages)
	fmt.Printf("Failed messages: %d\n", stats.FailedMessages)
	fmt.Printf("Total documents created: %d\n", stats.TotalDocuments)
	fmt.Printf("Documents stored: %d\n", stats.StoredDocuments)
	fmt.Printf("Documents failed: %d\n", stats.FailedDocuments)

	if len(stats.Errors) > 0 {
		fmt.Printf("\nErrors encountered: %d\n", len(stats.Errors))
		// Show first 10 errors
		for i, err := range stats.Errors {
			if i >= 10 {
				fmt.Printf("... and %d more errors\n", len(stats.Errors)-10)
				break
			}
			fmt.Printf("  - %v\n", err)
		}
	}

	if stats.ProcessedMessages > 0 {
		fmt.Printf("\nProcessing rate: %.2f messages/second\n", float64(stats.ProcessedMessages)/duration.Seconds())
	}
}

func printUsage() {
	fmt.Println("Connect LLM Data Ingestion Tool")
	fmt.Println("\nUsage:")
	fmt.Println("  ingest -input <path> [options]")
	fmt.Println("\nRequired:")
	fmt.Println("  -input string")
	fmt.Println("        Path to CSV file or directory to ingest")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  # Ingest a single CSV file")
	fmt.Println("  ingest -input slack/channel_general.csv")
	fmt.Println("\n  # Ingest all CSV files in a directory")
	fmt.Println("  ingest -input slack/")
	fmt.Println("\n  # Ingest with custom settings")
	fmt.Println("  ingest -input slack/ -batch-size 200 -concurrency 10")
}

// documentProcessorAdapter adapts processing.DocumentProcessor to ingestion.DocumentProcessor interface
type documentProcessorAdapter struct {
	processor *processing.DocumentProcessor
}

// ProcessMessage implements the ingestion.DocumentProcessor interface
func (a *documentProcessorAdapter) ProcessMessage(ctx context.Context, msg ingestion.SlackMessage) ([]vector.Document, error) {
	return a.processor.ProcessMessage(ctx, msg)
}
