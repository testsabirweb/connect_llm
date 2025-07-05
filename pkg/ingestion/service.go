package ingestion

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// DocumentProcessor interface to avoid import cycle
type DocumentProcessor interface {
	ProcessMessage(ctx context.Context, msg SlackMessage) ([]vector.Document, error)
}

// Service handles the complete ingestion pipeline
type Service struct {
	parser      *CSVParser
	processor   DocumentProcessor
	vectorStore vector.Client

	// Ingestion configuration
	batchSize        int
	maxConcurrency   int
	skipEmptyContent bool
}

// ServiceConfig contains configuration for the ingestion service
type ServiceConfig struct {
	BatchSize        int
	MaxConcurrency   int
	SkipEmptyContent bool
}

// DefaultServiceConfig returns default service configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		BatchSize:        100,
		MaxConcurrency:   5,
		SkipEmptyContent: true,
	}
}

// NewService creates a new ingestion service
func NewService(vectorStore vector.Client, processor DocumentProcessor, config ...ServiceConfig) *Service {
	cfg := DefaultServiceConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	parser := NewCSVParser(ParserConfig{
		BatchSize:       cfg.BatchSize,
		SkipErrors:      true,
		ValidateRecords: true,
	})

	return &Service{
		parser:           parser,
		processor:        processor,
		vectorStore:      vectorStore,
		batchSize:        cfg.BatchSize,
		maxConcurrency:   cfg.MaxConcurrency,
		skipEmptyContent: cfg.SkipEmptyContent,
	}
}

// IngestionStats tracks ingestion progress and statistics
type IngestionStats struct {
	TotalMessages     int
	ProcessedMessages int
	SkippedMessages   int
	FailedMessages    int
	TotalDocuments    int
	StoredDocuments   int
	FailedDocuments   int
	Errors            []error
	StartTime         time.Time
	EndTime           time.Time
	mu                sync.Mutex
}

// UpdateStats safely updates ingestion statistics
func (s *IngestionStats) UpdateStats(processed, skipped, failed, documents, storedDocs, failedDocs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ProcessedMessages += processed
	s.SkippedMessages += skipped
	s.FailedMessages += failed
	s.TotalDocuments += documents
	s.StoredDocuments += storedDocs
	s.FailedDocuments += failedDocs
}

// AddError adds an error to the stats
func (s *IngestionStats) AddError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Errors = append(s.Errors, err)
}

// GetSummary returns a summary of the ingestion stats
func (s *IngestionStats) GetSummary() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	duration := s.EndTime.Sub(s.StartTime)
	if s.EndTime.IsZero() {
		duration = time.Since(s.StartTime)
	}

	return map[string]interface{}{
		"total_messages":      s.TotalMessages,
		"processed_messages":  s.ProcessedMessages,
		"skipped_messages":    s.SkippedMessages,
		"failed_messages":     s.FailedMessages,
		"total_documents":     s.TotalDocuments,
		"stored_documents":    s.StoredDocuments,
		"failed_documents":    s.FailedDocuments,
		"error_count":         len(s.Errors),
		"duration_seconds":    duration.Seconds(),
		"messages_per_second": float64(s.ProcessedMessages) / duration.Seconds(),
	}
}

// IngestFile ingests a single CSV file
func (s *Service) IngestFile(ctx context.Context, filepath string) (*IngestionStats, error) {
	stats := &IngestionStats{
		StartTime: time.Now(),
	}

	// Create a worker pool for concurrent processing
	workerCount := s.maxConcurrency
	messageChan := make(chan []SlackMessage, workerCount)
	errorChan := make(chan error, workerCount)

	// Worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for messages := range messageChan {
				if err := s.processBatch(ctx, messages, stats); err != nil {
					errorChan <- err
				}
			}
		}()
	}

	// Error collector
	go func() {
		for err := range errorChan {
			stats.AddError(err)
		}
	}()

	// Parse file and send batches to workers
	err := s.parser.ParseFile(filepath, func(messages []SlackMessage, batchNum int) error {
		select {
		case messageChan <- messages:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}, func(processed, total, errors int) {
		stats.TotalMessages = total
		if processed%1000 == 0 {
			log.Printf("Progress: %d/%d messages processed, %d errors", processed, total, errors)
		}
	})

	// Close channels and wait for workers
	close(messageChan)
	wg.Wait()
	close(errorChan)

	stats.EndTime = time.Now()

	if err != nil {
		return stats, fmt.Errorf("failed to parse file: %w", err)
	}

	return stats, nil
}

// IngestDirectory ingests all CSV files in a directory
func (s *Service) IngestDirectory(ctx context.Context, dirPath string) (*IngestionStats, error) {
	totalStats := &IngestionStats{
		StartTime: time.Now(),
	}

	// Find all CSV files
	files, err := filepath.Glob(filepath.Join(dirPath, "*.csv"))
	if err != nil {
		return totalStats, fmt.Errorf("failed to list CSV files: %w", err)
	}

	if len(files) == 0 {
		return totalStats, fmt.Errorf("no CSV files found in %s", dirPath)
	}

	log.Printf("Found %d CSV files to process", len(files))

	// Process each file
	for i, file := range files {
		log.Printf("Processing file %d/%d: %s", i+1, len(files), filepath.Base(file))

		fileStats, err := s.IngestFile(ctx, file)
		if err != nil {
			totalStats.AddError(fmt.Errorf("failed to ingest %s: %w", file, err))
			continue
		}

		// Merge stats
		totalStats.UpdateStats(
			fileStats.ProcessedMessages,
			fileStats.SkippedMessages,
			fileStats.FailedMessages,
			fileStats.TotalDocuments,
			fileStats.StoredDocuments,
			fileStats.FailedDocuments,
		)
	}

	totalStats.EndTime = time.Now()
	return totalStats, nil
}

// processBatch processes a batch of messages
func (s *Service) processBatch(ctx context.Context, messages []SlackMessage, stats *IngestionStats) error {
	processed := 0
	skipped := 0
	failed := 0

	for _, msg := range messages {
		// Skip empty messages if configured
		if s.skipEmptyContent && msg.Content == "" && len(msg.FileIDs) == 0 {
			skipped++
			continue
		}

		// Process message into documents
		docs, err := s.processor.ProcessMessage(ctx, msg)
		if err != nil {
			failed++
			stats.AddError(fmt.Errorf("failed to process message %s: %w", msg.MessageID, err))
			continue
		}

		// Store documents
		storedCount := 0
		for _, doc := range docs {
			if err := s.vectorStore.Store(ctx, doc); err != nil {
				stats.AddError(fmt.Errorf("failed to store document %s: %w", doc.ID, err))
			} else {
				storedCount++
			}
		}

		stats.UpdateStats(0, 0, 0, len(docs), storedCount, len(docs)-storedCount)
		processed++
	}

	stats.UpdateStats(processed, skipped, failed, 0, 0, 0)
	return nil
}

// IngestRequest represents a request to ingest data
type IngestRequest struct {
	Type      string `json:"type"` // "file" or "directory"
	Path      string `json:"path"` // Path to file or directory
	BatchSize int    `json:"batch_size,omitempty"`
}

// IngestResponse represents the response from an ingestion operation
type IngestResponse struct {
	Success bool                   `json:"success"`
	Stats   map[string]interface{} `json:"stats"`
	Errors  []string               `json:"errors,omitempty"`
}
