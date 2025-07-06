package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/ollama"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// Service represents the main chat service
type Service struct {
	hub                 *Hub
	conversationManager *ConversationManager
	ragRetriever        *RAGRetriever
	promptBuilder       *PromptBuilder
	ollamaClient        *ollama.Client
	embedder            *embeddings.OllamaEmbedder
	config              ServiceConfig
	mu                  sync.RWMutex
}

// ServiceConfig holds configuration for the chat service
type ServiceConfig struct {
	OllamaURL         string
	OllamaModel       string
	MaxResponseTokens int
	StreamingEnabled  bool
	IncludeCitations  bool
	Temperature       float64
	EnableRAG         bool
	MinRAGScore       float64
}

// DefaultServiceConfig returns default service configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		OllamaURL:         "http://localhost:11434",
		OllamaModel:       "llama3:8b",
		MaxResponseTokens: 2000,
		StreamingEnabled:  true,
		IncludeCitations:  true,
		Temperature:       0.7,
		EnableRAG:         true,
		MinRAGScore:       0.5,
	}
}

// NewService creates a new chat service
func NewService(
	hub *Hub,
	vectorClient vector.Client,
	config ServiceConfig,
) *Service {
	// Create Ollama client
	ollamaClient := ollama.NewClient(config.OllamaURL)

	// Create embedder
	embedder := embeddings.NewOllamaEmbedder(config.OllamaURL, config.OllamaModel)

	// Create RAG retriever
	ragConfig := RAGConfig{
		MinScore: config.MinRAGScore,
	}
	ragRetriever := NewRAGRetriever(vectorClient, embedder, ragConfig)

	// Create prompt builder
	promptBuilder := NewPromptBuilder()

	// Create conversation manager
	conversationManager := NewConversationManager()

	service := &Service{
		hub:                 hub,
		conversationManager: conversationManager,
		ragRetriever:        ragRetriever,
		promptBuilder:       promptBuilder,
		ollamaClient:        ollamaClient,
		embedder:            embedder,
		config:              config,
	}

	// Set the service on the hub
	hub.SetChatService(service)

	return service
}

// HandleChatMessage processes a chat message from a client
func (s *Service) HandleChatMessage(ctx context.Context, client *Client, msg Message) {
	// Parse chat message
	var chatMsg ChatMessage
	if err := json.Unmarshal(msg.Metadata, &chatMsg); err != nil {
		s.sendError(client, msg.ID, "Invalid chat message format")
		return
	}

	// Get or create conversation
	conversation := s.conversationManager.GetOrCreateConversation(chatMsg.ConversationID, client.ID)

	// Add user message to conversation
	userMsg := ConversationMessage{
		ID:        uuid.New().String(),
		Role:      RoleUser,
		Content:   chatMsg.Query,
		Timestamp: time.Now(),
	}
	if err := s.conversationManager.AddMessage(conversation.ID, userMsg); err != nil {
		s.sendError(client, msg.ID, "Failed to save message")
		return
	}

	// Perform RAG retrieval if enabled
	var ragContext *RAGContext
	var err error
	if s.config.EnableRAG {
		ragContext, err = s.ragRetriever.RetrieveContext(ctx, chatMsg.Query)
		if err != nil {
			log.Printf("RAG retrieval error: %v", err)
			// Continue without RAG context
		}
	}

	// Get conversation history for context
	conversationHistory, err := s.conversationManager.GetContextMessages(
		conversation.ID,
		s.ragRetriever.config.MaxTokens,
	)
	if err != nil {
		log.Printf("Failed to get conversation history: %v", err)
		conversationHistory = []ConversationMessage{}
	}

	// Build prompt
	prompt := s.promptBuilder.BuildRAGPrompt(
		chatMsg.Query,
		ragContext,
		conversationHistory,
		chatMsg.IncludeCitations,
	)

	// Generate response
	if s.config.StreamingEnabled {
		s.streamResponse(ctx, client, msg.ID, conversation.ID, prompt, ragContext, chatMsg.IncludeCitations)
	} else {
		s.generateResponse(ctx, client, msg.ID, conversation.ID, prompt, ragContext, chatMsg.IncludeCitations)
	}
}

// streamResponse streams the LLM response to the client
func (s *Service) streamResponse(
	ctx context.Context,
	client *Client,
	messageID string,
	conversationID string,
	prompt []ollama.Message,
	ragContext *RAGContext,
	includeCitations bool,
) {
	// Create streaming request
	chatReq := ollama.ChatRequest{
		Model:    s.config.OllamaModel,
		Messages: prompt,
		Stream:   true,
		Options: &ollama.Options{
			Temperature: s.config.Temperature,
			NumPredict:  s.config.MaxResponseTokens,
		},
	}

	// Start streaming
	respChan, errChan := s.ollamaClient.ChatStream(ctx, chatReq)

	// Create response message ID
	responseID := uuid.New().String()
	var fullResponse strings.Builder

	// Send initial streaming message
	s.sendStreamingStart(client, messageID, responseID)

	// Process streaming chunks
	go func() {
		defer func() {
			// Send final message
			s.sendStreamingComplete(client, messageID, responseID, fullResponse.String())

			// Save assistant message to conversation
			assistantMsg := ConversationMessage{
				ID:        responseID,
				Role:      RoleAssistant,
				Content:   fullResponse.String(),
				Timestamp: time.Now(),
			}

			// Extract and add citations if enabled
			if includeCitations && ragContext != nil {
				citations := s.promptBuilder.ExtractCitationsFromResponse(fullResponse.String(), ragContext)
				assistantMsg.Citations = citations

				// Send citations to client
				if len(citations) > 0 {
					s.sendCitations(client, responseID, citations)
				}
			}

			if err := s.conversationManager.AddMessage(conversationID, assistantMsg); err != nil {
				log.Printf("Failed to save assistant message: %v", err)
			}
		}()

		for {
			select {
			case chunk, ok := <-respChan:
				if !ok {
					return
				}

				if chunk.Message.Content != "" {
					fullResponse.WriteString(chunk.Message.Content)
					s.sendStreamingChunk(client, messageID, responseID, chunk.Message.Content, chunk.Done)
				}

				if chunk.Done {
					return
				}

			case err := <-errChan:
				if err != nil {
					s.sendError(client, messageID, fmt.Sprintf("Streaming error: %v", err))
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// generateResponse generates a non-streaming response
func (s *Service) generateResponse(
	ctx context.Context,
	client *Client,
	messageID string,
	conversationID string,
	prompt []ollama.Message,
	ragContext *RAGContext,
	includeCitations bool,
) {
	// Create chat request
	chatReq := ollama.ChatRequest{
		Model:    s.config.OllamaModel,
		Messages: prompt,
		Stream:   false,
		Options: &ollama.Options{
			Temperature: s.config.Temperature,
			NumPredict:  s.config.MaxResponseTokens,
		},
	}

	// Generate response
	resp, err := s.ollamaClient.Chat(ctx, chatReq)
	if err != nil {
		s.sendError(client, messageID, fmt.Sprintf("Failed to generate response: %v", err))
		return
	}

	// Create response message
	responseID := uuid.New().String()
	assistantMsg := ConversationMessage{
		ID:        responseID,
		Role:      RoleAssistant,
		Content:   resp.Message.Content,
		Timestamp: time.Now(),
	}

	// Extract citations if enabled
	if includeCitations && ragContext != nil {
		citations := s.promptBuilder.ExtractCitationsFromResponse(resp.Message.Content, ragContext)
		assistantMsg.Citations = citations

		// Send citations to client
		if len(citations) > 0 {
			s.sendCitations(client, responseID, citations)
		}
	}

	// Save to conversation
	if err := s.conversationManager.AddMessage(conversationID, assistantMsg); err != nil {
		log.Printf("Failed to save assistant message: %v", err)
	}

	// Send response to client
	s.sendResponse(client, messageID, responseID, resp.Message.Content)
}

// Helper methods for sending messages

func (s *Service) sendError(client *Client, messageID, error string) {
	errorMsg := Message{
		Type:      MessageTypeError,
		ID:        messageID,
		Error:     error,
		Timestamp: time.Now(),
	}
	client.send <- errorMsg
}

func (s *Service) sendResponse(client *Client, requestID, responseID, content string) {
	respData, _ := json.Marshal(map[string]string{
		"response_id": responseID,
		"content":     content,
	})

	responseMsg := Message{
		Type:      MessageTypeResponse,
		ID:        requestID,
		Content:   content,
		Metadata:  respData,
		Timestamp: time.Now(),
	}
	client.send <- responseMsg
}

func (s *Service) sendStreamingStart(client *Client, requestID, responseID string) {
	streamData, _ := json.Marshal(StreamingResponse{
		MessageID: responseID,
		Chunk:     "",
		Done:      false,
	})

	msg := Message{
		Type:      MessageTypeStreaming,
		ID:        requestID,
		Metadata:  streamData,
		Timestamp: time.Now(),
	}
	client.send <- msg
}

func (s *Service) sendStreamingChunk(client *Client, requestID, responseID, chunk string, done bool) {
	streamData, _ := json.Marshal(StreamingResponse{
		MessageID: responseID,
		Chunk:     chunk,
		Done:      done,
	})

	msg := Message{
		Type:      MessageTypeStreaming,
		ID:        requestID,
		Metadata:  streamData,
		Timestamp: time.Now(),
	}
	client.send <- msg
}

func (s *Service) sendStreamingComplete(client *Client, requestID, responseID, fullContent string) {
	streamData, _ := json.Marshal(StreamingResponse{
		MessageID: responseID,
		Chunk:     "",
		Done:      true,
	})

	msg := Message{
		Type:      MessageTypeStreaming,
		ID:        requestID,
		Content:   fullContent,
		Metadata:  streamData,
		Timestamp: time.Now(),
	}
	client.send <- msg
}

func (s *Service) sendCitations(client *Client, messageID string, citations []Citation) {
	citationData, _ := json.Marshal(CitationResponse{
		MessageID: messageID,
		Citations: citations,
	})

	msg := Message{
		Type:      MessageTypeCitation,
		ID:        messageID,
		Metadata:  citationData,
		Timestamp: time.Now(),
	}
	client.send <- msg
}

// GetConversationHistory returns the conversation history
func (s *Service) GetConversationHistory(conversationID string) (*Conversation, error) {
	return s.conversationManager.GetConversation(conversationID)
}

// ExportConversation exports a conversation for persistence
func (s *Service) ExportConversation(conversationID string) (*Conversation, error) {
	return s.conversationManager.ExportConversation(conversationID)
}

// ImportConversation imports a previously exported conversation
func (s *Service) ImportConversation(conv *Conversation) error {
	return s.conversationManager.ImportConversation(conv)
}

// GetStats returns service statistics
func (s *Service) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"active_conversations": len(s.conversationManager.conversations),
		"connected_clients":    len(s.hub.clients),
		"rag_enabled":          s.config.EnableRAG,
		"streaming_enabled":    s.config.StreamingEnabled,
		"model":                s.config.OllamaModel,
	}

	return stats
}

// GetConversationManager returns the conversation manager
func (s *Service) GetConversationManager() *ConversationManager {
	return s.conversationManager
}
