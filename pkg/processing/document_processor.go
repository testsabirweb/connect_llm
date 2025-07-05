package processing

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/models"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// DocumentProcessor handles converting messages to documents with embeddings
type DocumentProcessor struct {
	embedder     *embeddings.OllamaEmbedder
	chunkSize    int
	chunkOverlap int
}

// NewDocumentProcessor creates a new document processor
func NewDocumentProcessor(embedder *embeddings.OllamaEmbedder, chunkSize, chunkOverlap int) *DocumentProcessor {
	return &DocumentProcessor{
		embedder:     embedder,
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// ProcessMessage converts a Slack message to one or more documents
func (p *DocumentProcessor) ProcessMessage(ctx context.Context, msg models.SlackMessage) ([]vector.Document, error) {
	// Skip empty messages
	if msg.Content == "" && len(msg.FileIDs) == 0 {
		return nil, nil
	}

	// Generate chunks if content is too long
	chunks := p.chunkText(msg.Content)
	if len(chunks) == 0 {
		chunks = []string{msg.Content} // At least one chunk even if empty
	}

	documents := make([]vector.Document, 0, len(chunks))

	for i, chunk := range chunks {
		// Generate unique ID for the chunk
		docID := p.generateDocumentID(msg.MessageID, i)

		// Generate embedding for the chunk
		embedding, err := p.embedder.GenerateEmbedding(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}

		// Create document
		doc := vector.Document{
			ID:        docID,
			Content:   chunk,
			Source:    "slack",
			SourceID:  msg.MessageID,
			Embedding: embedding,
			Metadata: vector.DocumentMetadata{
				Title:       p.generateTitle(msg),
				Author:      msg.User,
				CreatedAt:   msg.Timestamp,
				UpdatedAt:   msg.Timestamp,
				Permissions: p.extractPermissions(msg),
				Tags:        p.extractTags(msg),
				URL:         p.generateSlackURL(msg),
			},
		}

		documents = append(documents, doc)
	}

	return documents, nil
}

// ProcessMessages processes multiple messages into documents
func (p *DocumentProcessor) ProcessMessages(ctx context.Context, messages []models.SlackMessage) ([]vector.Document, error) {
	var allDocs []vector.Document

	for _, msg := range messages {
		docs, err := p.ProcessMessage(ctx, msg)
		if err != nil {
			return nil, fmt.Errorf("failed to process message %s: %w", msg.MessageID, err)
		}
		allDocs = append(allDocs, docs...)
	}

	return allDocs, nil
}

// chunkText splits text into chunks with overlap
func (p *DocumentProcessor) chunkText(text string) []string {
	if text == "" || len(text) <= p.chunkSize {
		return []string{text}
	}

	var chunks []string
	words := strings.Fields(text)

	for i := 0; i < len(words); {
		// Calculate chunk end
		end := i + p.chunkSize
		if end > len(words) {
			end = len(words)
		}

		// Create chunk
		chunk := strings.Join(words[i:end], " ")
		chunks = append(chunks, chunk)

		// Move to next chunk with overlap
		i += p.chunkSize - p.chunkOverlap
		if i >= len(words) {
			break
		}
	}

	return chunks
}

// generateDocumentID creates a unique ID for a document chunk
func (p *DocumentProcessor) generateDocumentID(messageID string, chunkIndex int) string {
	data := fmt.Sprintf("%s-%d", messageID, chunkIndex)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes of hash
}

// generateTitle creates a title for the document
func (p *DocumentProcessor) generateTitle(msg models.SlackMessage) string {
	// Use first 50 characters of content as title
	title := msg.Content
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	// If no content, use message type
	if title == "" {
		if msg.Subtype != "" {
			title = fmt.Sprintf("Slack %s message", msg.Subtype)
		} else {
			title = "Slack message"
		}
	}

	return title
}

// extractPermissions determines who can access this document
func (p *DocumentProcessor) extractPermissions(msg models.SlackMessage) []string {
	// For now, use channel ID as permission
	// In a real system, you'd map channels to user groups
	return []string{msg.Channel}
}

// extractTags generates tags for the document
func (p *DocumentProcessor) extractTags(msg models.SlackMessage) []string {
	tags := []string{"slack", msg.Channel}

	if msg.Type != "" {
		tags = append(tags, msg.Type)
	}

	if msg.Subtype != "" {
		tags = append(tags, msg.Subtype)
	}

	if msg.ThreadTS != "" {
		tags = append(tags, "thread")
	}

	if msg.ReplyCount > 0 {
		tags = append(tags, "has-replies")
	}

	return tags
}

// generateSlackURL creates a URL to the original message
// Note: This is a placeholder - real Slack URLs require workspace info
func (p *DocumentProcessor) generateSlackURL(msg models.SlackMessage) string {
	// Format: https://workspace.slack.com/archives/CHANNEL_ID/pTIMESTAMP
	timestamp := strings.Replace(msg.MessageID, ".", "", -1)
	return fmt.Sprintf("slack://channel/%s/message/%s", msg.Channel, timestamp)
}

// ChunkingConfig holds configuration for text chunking
type ChunkingConfig struct {
	MaxChunkSize int
	ChunkOverlap int
}

// DefaultChunkingConfig returns default chunking configuration
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		MaxChunkSize: 500, // 500 words per chunk
		ChunkOverlap: 50,  // 50 words overlap
	}
}
