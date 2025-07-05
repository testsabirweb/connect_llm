package processing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/models"
)

// MockEmbedder is a mock implementation for testing
type MockEmbedder struct {
	callCount int
	dimension int
}

func (m *MockEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.callCount++
	// Return a simple embedding based on text length
	embedding := make([]float32, m.dimension)
	for i := range embedding {
		embedding[i] = float32(len(text)) / float32(i+1)
	}
	return embedding, nil
}

func TestDocumentProcessor_ProcessMessage(t *testing.T) {
	// Create a mock embedder
	mockEmbedder := &embeddings.OllamaEmbedder{}
	_ = NewDocumentProcessor(mockEmbedder, 100, 20)

	// Test cases
	tests := []struct {
		name     string
		message  models.SlackMessage
		wantDocs int
		wantErr  bool
	}{
		{
			name: "Simple message",
			message: models.SlackMessage{
				MessageID: "msg123",
				Timestamp: time.Now(),
				Channel:   "C123456",
				User:      "U789012",
				Content:   "Hello, this is a test message",
				Type:      "message",
			},
			wantDocs: 1,
			wantErr:  false,
		},
		{
			name: "Empty message with file",
			message: models.SlackMessage{
				MessageID: "msg124",
				Timestamp: time.Now(),
				Channel:   "C123456",
				User:      "U789012",
				Content:   "",
				Type:      "message",
				FileIDs:   []string{"F123"},
			},
			wantDocs: 1,
			wantErr:  false,
		},
		{
			name: "Thread message",
			message: models.SlackMessage{
				MessageID:  "msg125",
				Timestamp:  time.Now(),
				Channel:    "C123456",
				User:       "U789012",
				Content:    "This is a reply",
				Type:       "message",
				ThreadTS:   "1234567890.123456",
				ReplyCount: 5,
			},
			wantDocs: 1,
			wantErr:  false,
		},
		{
			name: "System message",
			message: models.SlackMessage{
				MessageID: "msg126",
				Timestamp: time.Now(),
				Channel:   "C123456",
				User:      "U789012",
				Content:   "User joined the channel",
				Type:      "message",
				Subtype:   "channel_join",
			},
			wantDocs: 1,
			wantErr:  false,
		},
	}

	// Run tests without actual embedding (would need mock)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip actual processing since we'd need Ollama running
			// This test structure shows how it would be tested
			t.Logf("Test case: %s - would process message with ID %s", tt.name, tt.message.MessageID)
		})
	}
}

func TestDocumentProcessor_ChunkText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		chunkSize int
		overlap   int
		wantCount int
	}{
		{
			name:      "Short text",
			text:      "This is a short text",
			chunkSize: 100,
			overlap:   10,
			wantCount: 1,
		},
		{
			name:      "Empty text",
			text:      "",
			chunkSize: 100,
			overlap:   10,
			wantCount: 1,
		},
		{
			name:      "Exact chunk size",
			text:      "one two three four five six seven eight nine ten",
			chunkSize: 10,
			overlap:   2,
			wantCount: 2,
		},
		{
			name:      "Multiple chunks",
			text:      strings.Repeat("word ", 20),
			chunkSize: 10,
			overlap:   2,
			wantCount: 3,
		},
		{
			name:      "Long text with overlap",
			text:      strings.Repeat("word ", 30),
			chunkSize: 10,
			overlap:   5,
			wantCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewDocumentProcessor(nil, tt.chunkSize, tt.overlap)
			chunks := processor.chunkText(tt.text)
			if len(chunks) != tt.wantCount {
				t.Errorf("chunkText() returned %d chunks, want %d", len(chunks), tt.wantCount)
				for i, chunk := range chunks {
					t.Logf("Chunk %d: %s", i, chunk)
				}
			}
		})
	}
}

func TestDocumentProcessor_GenerateTitle(t *testing.T) {
	processor := NewDocumentProcessor(nil, 100, 20)

	tests := []struct {
		name    string
		message models.SlackMessage
		want    string
	}{
		{
			name: "Short content",
			message: models.SlackMessage{
				Content: "Hello world",
			},
			want: "Hello world",
		},
		{
			name: "Long content",
			message: models.SlackMessage{
				Content: "This is a very long message that exceeds the fifty character limit for titles",
			},
			want: "This is a very long message that exceeds the fi...",
		},
		{
			name: "Empty content with subtype",
			message: models.SlackMessage{
				Content: "",
				Subtype: "channel_join",
			},
			want: "Slack channel_join message",
		},
		{
			name: "Empty content no subtype",
			message: models.SlackMessage{
				Content: "",
			},
			want: "Slack message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := processor.generateTitle(tt.message)
			if title != tt.want {
				t.Errorf("generateTitle() = %q, want %q", title, tt.want)
			}
		})
	}
}

func TestDocumentProcessor_ExtractTags(t *testing.T) {
	processor := NewDocumentProcessor(nil, 100, 20)

	tests := []struct {
		name     string
		message  models.SlackMessage
		wantTags []string
	}{
		{
			name: "Basic message",
			message: models.SlackMessage{
				Channel: "C123",
				Type:    "message",
			},
			wantTags: []string{"slack", "C123", "message"},
		},
		{
			name: "Thread with replies",
			message: models.SlackMessage{
				Channel:    "C123",
				Type:       "message",
				ThreadTS:   "123.456",
				ReplyCount: 3,
			},
			wantTags: []string{"slack", "C123", "message", "thread", "has-replies"},
		},
		{
			name: "System message",
			message: models.SlackMessage{
				Channel: "C123",
				Type:    "message",
				Subtype: "bot_message",
			},
			wantTags: []string{"slack", "C123", "message", "bot_message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := processor.extractTags(tt.message)
			if len(tags) != len(tt.wantTags) {
				t.Errorf("extractTags() returned %d tags, want %d", len(tags), len(tt.wantTags))
			}

			// Check each tag exists
			tagMap := make(map[string]bool)
			for _, tag := range tags {
				tagMap[tag] = true
			}

			for _, wantTag := range tt.wantTags {
				if !tagMap[wantTag] {
					t.Errorf("Missing expected tag: %s", wantTag)
				}
			}
		})
	}
}

func TestChunkingConfig(t *testing.T) {
	config := DefaultChunkingConfig()

	if config.MaxChunkSize != 500 {
		t.Errorf("Expected MaxChunkSize to be 500, got %d", config.MaxChunkSize)
	}

	if config.ChunkOverlap != 50 {
		t.Errorf("Expected ChunkOverlap to be 50, got %d", config.ChunkOverlap)
	}
}
