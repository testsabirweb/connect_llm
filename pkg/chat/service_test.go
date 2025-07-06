package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// Mock implementations for testing

type mockVectorClient struct {
	documents []vector.Document
}

func (m *mockVectorClient) Initialize(ctx context.Context) error {
	return nil
}

func (m *mockVectorClient) Store(ctx context.Context, doc vector.Document) error {
	m.documents = append(m.documents, doc)
	return nil
}

func (m *mockVectorClient) Search(ctx context.Context, query []float32, limit int) ([]vector.Document, error) {
	// Return mock documents
	return m.documents[:min(limit, len(m.documents))], nil
}

func (m *mockVectorClient) SearchWithOptions(ctx context.Context, opts vector.SearchOptions) ([]vector.Document, error) {
	// Return mock documents based on options
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	return m.documents[:min(limit, len(m.documents))], nil
}

func (m *mockVectorClient) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockVectorClient) HealthCheck(ctx context.Context) error {
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Test functions

func TestNewService(t *testing.T) {
	hub := NewHub()
	vectorClient := &mockVectorClient{}
	config := DefaultServiceConfig()

	service := NewService(hub, vectorClient, config)

	if service == nil {
		t.Fatal("Expected service to be created")
	}

	if service.hub != hub {
		t.Error("Hub not set correctly")
	}

	if service.config.OllamaURL != config.OllamaURL {
		t.Error("Config not set correctly")
	}
}

func TestConversationManagement(t *testing.T) {
	manager := NewConversationManager()

	// Test creating conversation
	conv := manager.CreateConversation("test-client")
	if conv.ID == "" {
		t.Error("Expected conversation ID to be set")
	}
	if conv.ClientID != "test-client" {
		t.Error("Expected client ID to match")
	}

	// Test retrieving conversation
	retrieved, err := manager.GetConversation(conv.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve conversation: %v", err)
	}
	if retrieved.ID != conv.ID {
		t.Error("Retrieved conversation ID doesn't match")
	}

	// Test adding messages
	msg := ConversationMessage{
		ID:        uuid.New().String(),
		Role:      RoleUser,
		Content:   "Test message",
		Timestamp: time.Now(),
	}

	err = manager.AddMessage(conv.ID, msg)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	// Verify message was added
	updated, _ := manager.GetConversation(conv.ID)
	if len(updated.Messages) != 2 { // System message + user message
		t.Errorf("Expected 2 messages, got %d", len(updated.Messages))
	}
}

func TestRAGRetriever(t *testing.T) {
	// Create mock vector client with test documents
	vectorClient := &mockVectorClient{
		documents: []vector.Document{
			{
				ID:      "doc1",
				Content: "This is a test document about Go programming",
				Source:  "test",
				Metadata: vector.DocumentMetadata{
					Title:  "Go Programming",
					Author: "Test Author",
					Tags:   []string{"go", "programming"},
				},
			},
			{
				ID:      "doc2",
				Content: "Another document about web development",
				Source:  "test",
				Metadata: vector.DocumentMetadata{
					Title:  "Web Development",
					Author: "Test Author",
					Tags:   []string{"web", "development"},
				},
			},
		},
	}

	// Create mock embedder
	// Note: In a real test, you'd use a proper mock or test instance
	embedder := embeddings.NewOllamaEmbedder("http://localhost:11434", "llama3:8b")

	// Create RAG retriever
	retriever := NewRAGRetriever(vectorClient, embedder)

	// Test context retrieval
	ctx := context.Background()
	ragContext, err := retriever.RetrieveContext(ctx, "Tell me about Go programming")
	if err != nil {
		t.Fatalf("Failed to retrieve context: %v", err)
	}

	if ragContext == nil {
		t.Fatal("Expected non-nil context")
	}

	if ragContext.Query != "Tell me about Go programming" {
		t.Error("Query not set correctly")
	}

	// Should have retrieved some documents
	if len(ragContext.Documents) == 0 {
		t.Error("Expected to retrieve documents")
	}
}

func TestPromptBuilder(t *testing.T) {
	builder := NewPromptBuilder()

	// Create test RAG context
	ragContext := &RAGContext{
		Query: "Test query",
		Documents: []RetrievalResult{
			{
				Document: vector.Document{
					ID:      "doc1",
					Content: "Test document content",
					Metadata: vector.DocumentMetadata{
						Title:  "Test Title",
						Author: "Test Author",
					},
				},
				Score:     0.9,
				Relevance: "high",
			},
		},
	}

	// Create test conversation history
	history := []ConversationMessage{
		{
			Role:    RoleUser,
			Content: "Previous question",
		},
		{
			Role:    RoleAssistant,
			Content: "Previous answer",
		},
	}

	// Build RAG prompt
	messages := builder.BuildRAGPrompt("New question", ragContext, history, true)

	// Verify prompt structure
	if len(messages) < 3 {
		t.Errorf("Expected at least 3 messages, got %d", len(messages))
	}

	// Check system message
	if messages[0].Role != "system" {
		t.Error("First message should be system message")
	}

	// Check that history is included
	hasHistory := false
	for _, msg := range messages {
		if msg.Content == "Previous question" || msg.Content == "Previous answer" {
			hasHistory = true
			break
		}
	}
	if !hasHistory {
		t.Error("Expected conversation history to be included")
	}
}

func TestWebSocketMessage(t *testing.T) {
	// Test message serialization
	msg := Message{
		Type:      MessageTypeChat,
		ID:        "test-id",
		Content:   "Test content",
		Timestamp: time.Now(),
	}

	// Marshal to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Unmarshal back
	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Error("Message type doesn't match")
	}
	if decoded.ID != msg.ID {
		t.Error("Message ID doesn't match")
	}
}

func TestCitationExtraction(t *testing.T) {
	builder := NewPromptBuilder()

	// Create test context with documents
	ragContext := &RAGContext{
		Documents: []RetrievalResult{
			{Document: vector.Document{ID: "doc1"}},
			{Document: vector.Document{ID: "doc2"}},
			{Document: vector.Document{ID: "doc3"}},
		},
	}

	// Test response with citations
	response := "Based on the information [Document 1], we can see that [Document 3] also mentions this."

	citations := builder.ExtractCitationsFromResponse(response, ragContext)

	if len(citations) != 2 {
		t.Errorf("Expected 2 citations, got %d", len(citations))
	}

	// Check that correct documents were cited
	citedIDs := make(map[string]bool)
	for _, cit := range citations {
		citedIDs[cit.DocumentID] = true
	}

	if !citedIDs["doc1"] || !citedIDs["doc3"] {
		t.Error("Expected doc1 and doc3 to be cited")
	}
}

func TestConversationTokenManagement(t *testing.T) {
	config := ConversationConfig{
		MaxContextTokens: 100, // Very low for testing
		MaxMessages:      10,
	}
	manager := NewConversationManager(config)

	conv := manager.CreateConversation("test-client")

	// Add messages until we exceed token limit
	for i := 0; i < 20; i++ {
		msg := ConversationMessage{
			ID:        uuid.New().String(),
			Role:      RoleUser,
			Content:   strings.Repeat("word ", 20), // ~20 tokens
			Timestamp: time.Now(),
			Tokens:    20,
		}
		_ = manager.AddMessage(conv.ID, msg)
	}

	// Check that old messages were removed
	updated, _ := manager.GetConversation(conv.ID)
	if updated.TotalTokens > config.MaxContextTokens {
		t.Errorf("Total tokens %d exceeds limit %d", updated.TotalTokens, config.MaxContextTokens)
	}

	// Check that we don't exceed message limit
	if len(updated.Messages) > config.MaxMessages {
		t.Errorf("Message count %d exceeds limit %d", len(updated.Messages), config.MaxMessages)
	}
}
