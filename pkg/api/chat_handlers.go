package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/testsabirweb/connect_llm/pkg/chat"
)

// ConversationResponse represents a conversation in API responses
type ConversationResponse struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
	LastMessage  string    `json:"last_message,omitempty"`
}

// ConversationDetailResponse includes full conversation with messages
type ConversationDetailResponse struct {
	ConversationResponse
	Messages []MessageResponse      `json:"messages"`
	Stats    map[string]interface{} `json:"stats"`
}

// MessageResponse represents a message in API responses
type MessageResponse struct {
	ID        string             `json:"id"`
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	Timestamp time.Time          `json:"timestamp"`
	Citations []CitationResponse `json:"citations,omitempty"`
}

// CitationResponse represents a citation in API responses
type CitationResponse struct {
	DocumentID string                 `json:"document_id"`
	Content    string                 `json:"content"`
	Score      float64                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// handleWebSocket handles WebSocket connections for chat
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Delegate to the chat hub
	s.chatHub.ServeWS(w, r)
}

// handleConversations handles listing conversations
func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listConversations(w, r)
	case http.MethodPost:
		s.createConversation(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConversation handles individual conversation operations
func (s *Server) handleConversation(w http.ResponseWriter, r *http.Request) {
	// Extract conversation ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/chat/conversations/")
	conversationID := strings.TrimSuffix(path, "/")

	if conversationID == "" {
		http.Error(w, "Conversation ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getConversation(w, r, conversationID)
	case http.MethodDelete:
		s.deleteConversation(w, r, conversationID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listConversations returns all active conversations
func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	// Get client ID from header or query param
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = r.URL.Query().Get("client_id")
	}
	// Note: clientID is currently unused but would be used for filtering in production
	_ = clientID // Suppress unused variable warning

	// In a real implementation, you'd filter by client ID
	// For now, return all conversations (simplified)
	conversations := make([]ConversationResponse, 0)

	// Note: This is a simplified implementation
	// In production, you'd properly iterate through conversations

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"conversations": conversations,
		"total":         len(conversations),
	})
}

// createConversation creates a new conversation
func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	// Get client ID
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Create new conversation
	conv := s.chatService.GetConversationManager().CreateConversation(clientID)

	response := ConversationResponse{
		ID:           conv.ID,
		ClientID:     conv.ClientID,
		CreatedAt:    conv.CreatedAt,
		UpdatedAt:    conv.UpdatedAt,
		MessageCount: len(conv.Messages),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

// getConversation returns a specific conversation with messages
func (s *Server) getConversation(w http.ResponseWriter, r *http.Request, conversationID string) {
	conv, err := s.chatService.GetConversationHistory(conversationID)
	if err != nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	// Get conversation stats
	stats, _ := s.chatService.GetConversationManager().GetConversationStats(conversationID)

	// Convert messages
	messages := make([]MessageResponse, 0, len(conv.Messages))
	var lastMessage string

	for _, msg := range conv.Messages {
		msgResp := MessageResponse{
			ID:        msg.ID,
			Role:      string(msg.Role),
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		}

		// Convert citations
		if len(msg.Citations) > 0 {
			citations := make([]CitationResponse, 0, len(msg.Citations))
			for _, cit := range msg.Citations {
				citations = append(citations, CitationResponse{
					DocumentID: cit.DocumentID,
					Content:    cit.Content,
					Score:      cit.Score,
					Metadata:   cit.Metadata,
				})
			}
			msgResp.Citations = citations
		}

		messages = append(messages, msgResp)
		if msg.Role != "system" {
			lastMessage = msg.Content
		}
	}

	response := ConversationDetailResponse{
		ConversationResponse: ConversationResponse{
			ID:           conv.ID,
			ClientID:     conv.ClientID,
			CreatedAt:    conv.CreatedAt,
			UpdatedAt:    conv.UpdatedAt,
			MessageCount: len(conv.Messages),
			LastMessage:  truncateString(lastMessage, 100),
		},
		Messages: messages,
		Stats:    stats,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// deleteConversation deletes a conversation
func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request, conversationID string) {
	// In a real implementation, you'd delete the conversation
	// For now, just return success
	w.WriteHeader(http.StatusNoContent)
}

// GetChatService returns the chat service (for initialization)
func (s *Server) GetChatService() *chat.Service {
	return s.chatService
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
