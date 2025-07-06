package chat

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Role represents the role of a message sender
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// ConversationMessage represents a single message in a conversation
type ConversationMessage struct {
	ID        string                 `json:"id"`
	Role      Role                   `json:"role"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Citations []Citation             `json:"citations,omitempty"`
	Tokens    int                    `json:"tokens"`
}

// Conversation represents a chat conversation with history
type Conversation struct {
	ID               string                 `json:"id"`
	ClientID         string                 `json:"client_id"`
	Messages         []ConversationMessage  `json:"messages"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	TotalTokens      int                    `json:"total_tokens"`
	MaxContextTokens int                    `json:"max_context_tokens"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	mu               sync.RWMutex
}

// ConversationConfig holds configuration for conversation management
type ConversationConfig struct {
	MaxContextTokens       int           // Maximum tokens in context window
	MaxMessages            int           // Maximum messages to keep in memory
	SystemPrompt           string        // Default system prompt
	MessageRetentionPeriod time.Duration // How long to keep messages
	CompressOldMessages    bool          // Whether to compress old messages
	CompressionThreshold   int           // Number of messages before compression
}

// DefaultConversationConfig returns default conversation configuration
func DefaultConversationConfig() ConversationConfig {
	return ConversationConfig{
		MaxContextTokens:       8000, // Conservative default for most models
		MaxMessages:            100,  // Keep last 100 messages in memory
		SystemPrompt:           "You are a helpful assistant with access to a knowledge base. Use the provided context to answer questions accurately.",
		MessageRetentionPeriod: 24 * time.Hour,
		CompressOldMessages:    true,
		CompressionThreshold:   20,
	}
}

// ConversationManager manages multiple conversations
type ConversationManager struct {
	conversations map[string]*Conversation
	config        ConversationConfig
	mu            sync.RWMutex
}

// NewConversationManager creates a new conversation manager
func NewConversationManager(config ...ConversationConfig) *ConversationManager {
	cfg := DefaultConversationConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return &ConversationManager{
		conversations: make(map[string]*Conversation),
		config:        cfg,
	}
}

// CreateConversation creates a new conversation
func (m *ConversationManager) CreateConversation(clientID string) *Conversation {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv := &Conversation{
		ID:               uuid.New().String(),
		ClientID:         clientID,
		Messages:         make([]ConversationMessage, 0),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		TotalTokens:      0,
		MaxContextTokens: m.config.MaxContextTokens,
		Metadata:         make(map[string]interface{}),
	}

	// Add system prompt if configured
	if m.config.SystemPrompt != "" {
		systemMsg := ConversationMessage{
			ID:        uuid.New().String(),
			Role:      RoleSystem,
			Content:   m.config.SystemPrompt,
			Timestamp: time.Now(),
			Tokens:    m.estimateTokens(m.config.SystemPrompt),
		}
		conv.Messages = append(conv.Messages, systemMsg)
		conv.TotalTokens += systemMsg.Tokens
	}

	m.conversations[conv.ID] = conv
	return conv
}

// GetConversation retrieves a conversation by ID
func (m *ConversationManager) GetConversation(conversationID string) (*Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[conversationID]
	if !exists {
		return nil, fmt.Errorf("conversation %s not found", conversationID)
	}

	return conv, nil
}

// GetOrCreateConversation gets an existing conversation or creates a new one
func (m *ConversationManager) GetOrCreateConversation(conversationID, clientID string) *Conversation {
	if conversationID != "" {
		if conv, err := m.GetConversation(conversationID); err == nil {
			return conv
		}
	}

	return m.CreateConversation(clientID)
}

// AddMessage adds a message to a conversation
func (m *ConversationManager) AddMessage(conversationID string, msg ConversationMessage) error {
	m.mu.RLock()
	conv, exists := m.conversations[conversationID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("conversation %s not found", conversationID)
	}

	conv.mu.Lock()
	defer conv.mu.Unlock()

	// Estimate tokens if not provided
	if msg.Tokens == 0 {
		msg.Tokens = m.estimateTokens(msg.Content)
	}

	// Add message
	conv.Messages = append(conv.Messages, msg)
	conv.TotalTokens += msg.Tokens
	conv.UpdatedAt = time.Now()

	// Manage context window
	m.manageContextWindow(conv)

	// Apply compression if needed
	if m.config.CompressOldMessages && len(conv.Messages) > m.config.CompressionThreshold {
		m.compressOldMessages(conv)
	}

	return nil
}

// GetContextMessages returns messages that fit within the context window
func (m *ConversationManager) GetContextMessages(conversationID string, additionalTokens int) ([]ConversationMessage, error) {
	conv, err := m.GetConversation(conversationID)
	if err != nil {
		return nil, err
	}

	conv.mu.RLock()
	defer conv.mu.RUnlock()

	availableTokens := conv.MaxContextTokens - additionalTokens
	if availableTokens <= 0 {
		return nil, fmt.Errorf("no token budget available for context")
	}

	// Work backwards to include as many recent messages as possible
	contextMessages := make([]ConversationMessage, 0)
	tokenCount := 0

	// Always include system message if present
	systemMsgIncluded := false
	if len(conv.Messages) > 0 && conv.Messages[0].Role == RoleSystem {
		contextMessages = append(contextMessages, conv.Messages[0])
		tokenCount += conv.Messages[0].Tokens
		systemMsgIncluded = true
	}

	// Add messages from most recent backwards
	for i := len(conv.Messages) - 1; i >= 0; i-- {
		msg := conv.Messages[i]

		// Skip system message if already included
		if i == 0 && systemMsgIncluded {
			continue
		}

		if tokenCount+msg.Tokens > availableTokens {
			break
		}

		contextMessages = append([]ConversationMessage{msg}, contextMessages...)
		tokenCount += msg.Tokens
	}

	return contextMessages, nil
}

// manageContextWindow ensures the conversation stays within token limits
func (m *ConversationManager) manageContextWindow(conv *Conversation) {
	// If total tokens exceed limit, remove old messages (except system)
	if conv.TotalTokens > conv.MaxContextTokens {
		startIdx := 0
		if len(conv.Messages) > 0 && conv.Messages[0].Role == RoleSystem {
			startIdx = 1 // Keep system message
		}

		// Remove messages until we're under the limit
		for conv.TotalTokens > conv.MaxContextTokens && startIdx < len(conv.Messages)-1 {
			conv.TotalTokens -= conv.Messages[startIdx].Tokens
			startIdx++
		}

		// Create new slice with remaining messages
		newMessages := make([]ConversationMessage, 0)
		if len(conv.Messages) > 0 && conv.Messages[0].Role == RoleSystem {
			newMessages = append(newMessages, conv.Messages[0])
		}
		newMessages = append(newMessages, conv.Messages[startIdx:]...)
		conv.Messages = newMessages
	}

	// Also apply message count limit
	if len(conv.Messages) > m.config.MaxMessages {
		startIdx := len(conv.Messages) - m.config.MaxMessages
		if len(conv.Messages) > 0 && conv.Messages[0].Role == RoleSystem {
			// Keep system message
			newMessages := []ConversationMessage{conv.Messages[0]}
			newMessages = append(newMessages, conv.Messages[startIdx+1:]...)
			conv.Messages = newMessages
		} else {
			conv.Messages = conv.Messages[startIdx:]
		}

		// Recalculate total tokens
		conv.TotalTokens = 0
		for _, msg := range conv.Messages {
			conv.TotalTokens += msg.Tokens
		}
	}
}

// compressOldMessages compresses older messages to save space
func (m *ConversationManager) compressOldMessages(conv *Conversation) {
	// This is a placeholder for message compression logic
	// In a real implementation, you might:
	// 1. Summarize old conversations
	// 2. Store full history in a database
	// 3. Keep only summaries in memory
}

// CleanupOldConversations removes conversations older than retention period
func (m *ConversationManager) CleanupOldConversations() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.config.MessageRetentionPeriod)

	for id, conv := range m.conversations {
		if conv.UpdatedAt.Before(cutoff) {
			delete(m.conversations, id)
		}
	}
}

// ExportConversation exports a conversation for persistence
func (m *ConversationManager) ExportConversation(conversationID string) (*Conversation, error) {
	conv, err := m.GetConversation(conversationID)
	if err != nil {
		return nil, err
	}

	conv.mu.RLock()
	defer conv.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	exportedConv := &Conversation{
		ID:               conv.ID,
		ClientID:         conv.ClientID,
		Messages:         make([]ConversationMessage, len(conv.Messages)),
		CreatedAt:        conv.CreatedAt,
		UpdatedAt:        conv.UpdatedAt,
		TotalTokens:      conv.TotalTokens,
		MaxContextTokens: conv.MaxContextTokens,
		Metadata:         make(map[string]interface{}),
	}

	copy(exportedConv.Messages, conv.Messages)

	for k, v := range conv.Metadata {
		exportedConv.Metadata[k] = v
	}

	return exportedConv, nil
}

// ImportConversation imports a previously exported conversation
func (m *ConversationManager) ImportConversation(conv *Conversation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.conversations[conv.ID] = conv
	return nil
}

// estimateTokens estimates the number of tokens in a text
func (m *ConversationManager) estimateTokens(text string) int {
	// Simple estimation: ~4 characters per token
	// In production, use a proper tokenizer
	return len(text) / 4
}

// GetConversationStats returns statistics about a conversation
func (m *ConversationManager) GetConversationStats(conversationID string) (map[string]interface{}, error) {
	conv, err := m.GetConversation(conversationID)
	if err != nil {
		return nil, err
	}

	conv.mu.RLock()
	defer conv.mu.RUnlock()

	stats := map[string]interface{}{
		"id":                 conv.ID,
		"message_count":      len(conv.Messages),
		"total_tokens":       conv.TotalTokens,
		"max_tokens":         conv.MaxContextTokens,
		"token_usage":        float64(conv.TotalTokens) / float64(conv.MaxContextTokens) * 100,
		"created_at":         conv.CreatedAt,
		"updated_at":         conv.UpdatedAt,
		"duration":           time.Since(conv.CreatedAt).String(),
		"user_messages":      0,
		"assistant_messages": 0,
		"system_messages":    0,
	}

	// Count messages by role
	for _, msg := range conv.Messages {
		switch msg.Role {
		case RoleUser:
			stats["user_messages"] = stats["user_messages"].(int) + 1
		case RoleAssistant:
			stats["assistant_messages"] = stats["assistant_messages"].(int) + 1
		case RoleSystem:
			stats["system_messages"] = stats["system_messages"].(int) + 1
		}
	}

	return stats, nil
}
