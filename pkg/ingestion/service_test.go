package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// mockDocumentProcessor implements the DocumentProcessor interface for testing
type mockDocumentProcessor struct {
	processFunc func(context.Context, SlackMessage) ([]vector.Document, error)
	callCount   int
}

func (m *mockDocumentProcessor) ProcessMessage(ctx context.Context, msg SlackMessage) ([]vector.Document, error) {
	m.callCount++
	if m.processFunc != nil {
		return m.processFunc(ctx, msg)
	}
	// Default behavior: create one document per message
	return []vector.Document{
		{
			ID:       msg.MessageID,
			Content:  msg.Content,
			Source:   "slack",
			SourceID: msg.MessageID,
			Metadata: vector.DocumentMetadata{
				Title:     msg.Content[:min(len(msg.Content), 50)],
				Author:    msg.User,
				CreatedAt: msg.Timestamp,
			},
		},
	}, nil
}

// mockVectorClient implements the vector.Client interface for testing
type mockVectorClient struct {
	storeFunc   func(context.Context, vector.Document) error
	storeCount  int
	storeErrors []error
}

func (m *mockVectorClient) Initialize(ctx context.Context) error {
	return nil
}

func (m *mockVectorClient) Store(ctx context.Context, doc vector.Document) error {
	m.storeCount++
	if m.storeFunc != nil {
		return m.storeFunc(ctx, doc)
	}
	if len(m.storeErrors) > 0 && m.storeCount <= len(m.storeErrors) {
		return m.storeErrors[m.storeCount-1]
	}
	return nil
}

func (m *mockVectorClient) Search(ctx context.Context, query []float32, limit int) ([]vector.Document, error) {
	return nil, nil
}

func (m *mockVectorClient) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockVectorClient) HealthCheck(ctx context.Context) error {
	return nil
}

func TestNewService(t *testing.T) {
	mockVector := &mockVectorClient{}
	mockProcessor := &mockDocumentProcessor{}

	tests := []struct {
		name   string
		config ServiceConfig
		want   ServiceConfig
	}{
		{
			name: "default config",
			want: DefaultServiceConfig(),
		},
		{
			name: "custom config",
			config: ServiceConfig{
				BatchSize:        50,
				MaxConcurrency:   10,
				SkipEmptyContent: false,
			},
			want: ServiceConfig{
				BatchSize:        50,
				MaxConcurrency:   10,
				SkipEmptyContent: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *Service
			if tt.name == "default config" {
				service = NewService(mockVector, mockProcessor)
			} else {
				service = NewService(mockVector, mockProcessor, tt.config)
			}

			if service.batchSize != tt.want.BatchSize {
				t.Errorf("batchSize = %d, want %d", service.batchSize, tt.want.BatchSize)
			}
			if service.maxConcurrency != tt.want.MaxConcurrency {
				t.Errorf("maxConcurrency = %d, want %d", service.maxConcurrency, tt.want.MaxConcurrency)
			}
			if service.skipEmptyContent != tt.want.SkipEmptyContent {
				t.Errorf("skipEmptyContent = %v, want %v", service.skipEmptyContent, tt.want.SkipEmptyContent)
			}
		})
	}
}

func TestIngestionStats(t *testing.T) {
	stats := &IngestionStats{
		StartTime: time.Now(),
	}

	// Test UpdateStats
	stats.UpdateStats(10, 2, 1, 15, 14, 1)

	if stats.ProcessedMessages != 10 {
		t.Errorf("ProcessedMessages = %d, want 10", stats.ProcessedMessages)
	}
	if stats.SkippedMessages != 2 {
		t.Errorf("SkippedMessages = %d, want 2", stats.SkippedMessages)
	}
	if stats.FailedMessages != 1 {
		t.Errorf("FailedMessages = %d, want 1", stats.FailedMessages)
	}
	if stats.TotalDocuments != 15 {
		t.Errorf("TotalDocuments = %d, want 15", stats.TotalDocuments)
	}
	if stats.StoredDocuments != 14 {
		t.Errorf("StoredDocuments = %d, want 14", stats.StoredDocuments)
	}
	if stats.FailedDocuments != 1 {
		t.Errorf("FailedDocuments = %d, want 1", stats.FailedDocuments)
	}

	// Test AddError
	err1 := errors.New("test error 1")
	err2 := errors.New("test error 2")
	stats.AddError(err1)
	stats.AddError(err2)

	if len(stats.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(stats.Errors))
	}

	// Test GetSummary
	stats.EndTime = stats.StartTime.Add(10 * time.Second)
	summary := stats.GetSummary()

	if summary["total_messages"] != 0 { // TotalMessages wasn't set
		t.Errorf("total_messages = %v, want 0", summary["total_messages"])
	}
	if summary["processed_messages"] != 10 {
		t.Errorf("processed_messages = %v, want 10", summary["processed_messages"])
	}
	if summary["error_count"] != 2 {
		t.Errorf("error_count = %v, want 2", summary["error_count"])
	}
	if summary["duration_seconds"].(float64) < 9 || summary["duration_seconds"].(float64) > 11 {
		t.Errorf("duration_seconds = %v, want ~10", summary["duration_seconds"])
	}
}

func TestProcessBatch(t *testing.T) {
	tests := []struct {
		name             string
		messages         []SlackMessage
		skipEmptyContent bool
		processFunc      func(context.Context, SlackMessage) ([]vector.Document, error)
		storeFunc        func(context.Context, vector.Document) error
		wantProcessed    int
		wantSkipped      int
		wantFailed       int
		wantDocs         int
		wantStored       int
		wantErrors       int
	}{
		{
			name: "successful processing",
			messages: []SlackMessage{
				{MessageID: "1", Content: "Hello", User: "user1"},
				{MessageID: "2", Content: "World", User: "user2"},
			},
			skipEmptyContent: true,
			wantProcessed:    2,
			wantSkipped:      0,
			wantFailed:       0,
			wantDocs:         2,
			wantStored:       2,
			wantErrors:       0,
		},
		{
			name: "skip empty content",
			messages: []SlackMessage{
				{MessageID: "1", Content: "Hello", User: "user1"},
				{MessageID: "2", Content: "", User: "user2"},
				{MessageID: "3", Content: "World", User: "user3"},
			},
			skipEmptyContent: true,
			wantProcessed:    2,
			wantSkipped:      1,
			wantFailed:       0,
			wantDocs:         2,
			wantStored:       2,
			wantErrors:       0,
		},
		{
			name: "don't skip empty content",
			messages: []SlackMessage{
				{MessageID: "1", Content: "Hello", User: "user1"},
				{MessageID: "2", Content: "", User: "user2"},
			},
			skipEmptyContent: false,
			wantProcessed:    2,
			wantSkipped:      0,
			wantFailed:       0,
			wantDocs:         2,
			wantStored:       2,
			wantErrors:       0,
		},
		{
			name: "processing error",
			messages: []SlackMessage{
				{MessageID: "1", Content: "Hello", User: "user1"},
				{MessageID: "2", Content: "Error", User: "user2"},
			},
			skipEmptyContent: true,
			processFunc: func(ctx context.Context, msg SlackMessage) ([]vector.Document, error) {
				if msg.Content == "Error" {
					return nil, errors.New("processing error")
				}
				return []vector.Document{{ID: msg.MessageID, Content: msg.Content}}, nil
			},
			wantProcessed: 1,
			wantSkipped:   0,
			wantFailed:    1,
			wantDocs:      1,
			wantStored:    1,
			wantErrors:    1,
		},
		{
			name: "storage error",
			messages: []SlackMessage{
				{MessageID: "1", Content: "Hello", User: "user1"},
				{MessageID: "2", Content: "World", User: "user2"},
			},
			skipEmptyContent: true,
			storeFunc: func(ctx context.Context, doc vector.Document) error {
				if doc.ID == "2" {
					return errors.New("storage error")
				}
				return nil
			},
			wantProcessed: 2,
			wantSkipped:   0,
			wantFailed:    0,
			wantDocs:      2,
			wantStored:    1,
			wantErrors:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProcessor := &mockDocumentProcessor{processFunc: tt.processFunc}
			mockVector := &mockVectorClient{storeFunc: tt.storeFunc}

			service := &Service{
				processor:        mockProcessor,
				vectorStore:      mockVector,
				skipEmptyContent: tt.skipEmptyContent,
			}

			stats := &IngestionStats{}
			ctx := context.Background()

			err := service.processBatch(ctx, tt.messages, stats)
			if err != nil {
				t.Errorf("processBatch() error = %v", err)
			}

			if stats.ProcessedMessages != tt.wantProcessed {
				t.Errorf("ProcessedMessages = %d, want %d", stats.ProcessedMessages, tt.wantProcessed)
			}
			if stats.SkippedMessages != tt.wantSkipped {
				t.Errorf("SkippedMessages = %d, want %d", stats.SkippedMessages, tt.wantSkipped)
			}
			if stats.FailedMessages != tt.wantFailed {
				t.Errorf("FailedMessages = %d, want %d", stats.FailedMessages, tt.wantFailed)
			}
			if stats.TotalDocuments != tt.wantDocs {
				t.Errorf("TotalDocuments = %d, want %d", stats.TotalDocuments, tt.wantDocs)
			}
			if stats.StoredDocuments != tt.wantStored {
				t.Errorf("StoredDocuments = %d, want %d", stats.StoredDocuments, tt.wantStored)
			}
			if len(stats.Errors) != tt.wantErrors {
				t.Errorf("len(Errors) = %d, want %d", len(stats.Errors), tt.wantErrors)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
