package chat

import (
	"fmt"
	"strings"

	"github.com/testsabirweb/connect_llm/pkg/ollama"
)

// PromptTemplate represents a template for generating prompts
type PromptTemplate struct {
	SystemPrompt        string
	ContextFormat       string
	QueryFormat         string
	ResponseFormat      string
	CitationInstruction string
}

// DefaultPromptTemplate returns the default prompt template
func DefaultPromptTemplate() PromptTemplate {
	return PromptTemplate{
		SystemPrompt: `You are a helpful AI assistant with access to a knowledge base.
Your responses should be:
1. Accurate and based on the provided context
2. Clear and well-structured
3. Include citations when referencing specific information
4. Acknowledge when information is not available in the context`,

		ContextFormat: `Here is the relevant context from the knowledge base:

{{CONTEXT}}

End of context.`,

		QueryFormat: `User Query: {{QUERY}}`,

		ResponseFormat: `Based on the provided context, please answer the user's query.
If the context doesn't contain relevant information, acknowledge this and provide the best general answer you can.`,

		CitationInstruction: `When referencing information from the context, use [Document X] format for citations.`,
	}
}

// PromptBuilder builds prompts for the LLM
type PromptBuilder struct {
	template PromptTemplate
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder(template ...PromptTemplate) *PromptBuilder {
	tmpl := DefaultPromptTemplate()
	if len(template) > 0 {
		tmpl = template[0]
	}

	return &PromptBuilder{
		template: tmpl,
	}
}

// BuildRAGPrompt builds a complete RAG prompt with context
func (b *PromptBuilder) BuildRAGPrompt(
	query string,
	ragContext *RAGContext,
	conversationHistory []ConversationMessage,
	includeCitations bool,
) []ollama.Message {
	messages := make([]ollama.Message, 0)

	// Add system prompt
	systemPrompt := b.template.SystemPrompt
	if includeCitations {
		systemPrompt += "\n\n" + b.template.CitationInstruction
	}

	messages = append(messages, ollama.Message{
		Role:    string(RoleSystem),
		Content: systemPrompt,
	})

	// Add conversation history (if any)
	messages = append(messages, b.buildHistoryMessages(conversationHistory)...)

	// Build the current query with context
	userMessage := b.buildUserMessage(query, ragContext)
	messages = append(messages, ollama.Message{
		Role:    string(RoleUser),
		Content: userMessage,
	})

	return messages
}

// BuildSimplePrompt builds a prompt without RAG context
func (b *PromptBuilder) BuildSimplePrompt(
	query string,
	conversationHistory []ConversationMessage,
) []ollama.Message {
	messages := make([]ollama.Message, 0)

	// Add system prompt
	messages = append(messages, ollama.Message{
		Role:    string(RoleSystem),
		Content: b.template.SystemPrompt,
	})

	// Add conversation history
	messages = append(messages, b.buildHistoryMessages(conversationHistory)...)

	// Add current query
	messages = append(messages, ollama.Message{
		Role:    string(RoleUser),
		Content: query,
	})

	return messages
}

// buildHistoryMessages converts conversation history to Ollama messages
func (b *PromptBuilder) buildHistoryMessages(history []ConversationMessage) []ollama.Message {
	messages := make([]ollama.Message, 0, len(history))

	for _, msg := range history {
		// Skip system messages as they're handled separately
		if msg.Role == RoleSystem {
			continue
		}

		messages = append(messages, ollama.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	return messages
}

// buildUserMessage builds the user message with RAG context
func (b *PromptBuilder) buildUserMessage(query string, ragContext *RAGContext) string {
	var sb strings.Builder

	// Add context if available
	if ragContext != nil && len(ragContext.Documents) > 0 {
		contextStr := b.formatRAGContext(ragContext)
		contextSection := strings.Replace(b.template.ContextFormat, "{{CONTEXT}}", contextStr, 1)
		sb.WriteString(contextSection)
		sb.WriteString("\n\n")
	}

	// Add the query
	querySection := strings.Replace(b.template.QueryFormat, "{{QUERY}}", query, 1)
	sb.WriteString(querySection)
	sb.WriteString("\n\n")

	// Add response format instructions
	sb.WriteString(b.template.ResponseFormat)

	return sb.String()
}

// formatRAGContext formats the RAG context for inclusion in the prompt
func (b *PromptBuilder) formatRAGContext(context *RAGContext) string {
	var sb strings.Builder

	for i, result := range context.Documents {
		sb.WriteString(fmt.Sprintf("Document %d:\n", i+1))

		// Add metadata if available
		if result.Document.Metadata.Title != "" {
			sb.WriteString(fmt.Sprintf("Title: %s\n", result.Document.Metadata.Title))
		}
		if result.Document.Metadata.Author != "" {
			sb.WriteString(fmt.Sprintf("Author: %s\n", result.Document.Metadata.Author))
		}

		// Add content
		sb.WriteString(fmt.Sprintf("Content: %s\n", result.Document.Content))

		// Add relevance score
		sb.WriteString(fmt.Sprintf("Relevance: %s (score: %.2f)\n", result.Relevance, result.Score))

		sb.WriteString("\n---\n\n")
	}

	return strings.TrimSuffix(sb.String(), "\n---\n\n")
}

// ExtractCitationsFromResponse extracts citation references from the LLM response
func (b *PromptBuilder) ExtractCitationsFromResponse(response string, ragContext *RAGContext) []Citation {
	citations := make([]Citation, 0)

	// Simple pattern matching for [Document X] format
	// In production, you might use a more sophisticated approach
	for i, result := range ragContext.Documents {
		docRef := fmt.Sprintf("[Document %d]", i+1)
		if strings.Contains(response, docRef) {
			citation := Citation{
				DocumentID: result.Document.ID,
				Content:    result.Document.Content,
				Score:      result.Score,
				Metadata: map[string]interface{}{
					"title":  result.Document.Metadata.Title,
					"author": result.Document.Metadata.Author,
					"source": result.Document.Source,
				},
			}
			citations = append(citations, citation)
		}
	}

	return citations
}

// PromptConfig holds configuration for prompt generation
type PromptConfig struct {
	MaxContextDocuments int
	IncludeMetadata     bool
	IncludeCitations    bool
	ContextTokenLimit   int
}

// DefaultPromptConfig returns default prompt configuration
func DefaultPromptConfig() PromptConfig {
	return PromptConfig{
		MaxContextDocuments: 5,
		IncludeMetadata:     true,
		IncludeCitations:    true,
		ContextTokenLimit:   4000,
	}
}

// AdvancedPromptBuilder provides more control over prompt generation
type AdvancedPromptBuilder struct {
	*PromptBuilder
	config PromptConfig
}

// NewAdvancedPromptBuilder creates an advanced prompt builder
func NewAdvancedPromptBuilder(config PromptConfig, template ...PromptTemplate) *AdvancedPromptBuilder {
	tmpl := DefaultPromptTemplate()
	if len(template) > 0 {
		tmpl = template[0]
	}

	return &AdvancedPromptBuilder{
		PromptBuilder: NewPromptBuilder(tmpl),
		config:        config,
	}
}

// BuildOptimizedRAGPrompt builds an optimized RAG prompt with token management
func (ab *AdvancedPromptBuilder) BuildOptimizedRAGPrompt(
	query string,
	ragContext *RAGContext,
	conversationHistory []ConversationMessage,
) ([]ollama.Message, *PromptMetadata) {
	metadata := &PromptMetadata{
		TotalTokens:       0,
		ContextTokens:     0,
		HistoryTokens:     0,
		DocumentsIncluded: 0,
		TruncatedHistory:  false,
		TruncatedContext:  false,
	}

	// Limit documents to configured maximum
	documentsToInclude := ragContext.Documents
	if len(documentsToInclude) > ab.config.MaxContextDocuments {
		documentsToInclude = documentsToInclude[:ab.config.MaxContextDocuments]
		metadata.TruncatedContext = true
	}
	metadata.DocumentsIncluded = len(documentsToInclude)

	// Create limited context
	limitedContext := &RAGContext{
		Query:       ragContext.Query,
		Documents:   documentsToInclude,
		TotalTokens: 0,
		Metadata:    ragContext.Metadata,
	}

	// Calculate tokens for context
	for _, doc := range documentsToInclude {
		limitedContext.TotalTokens += doc.TokenCount
	}
	metadata.ContextTokens = limitedContext.TotalTokens

	// Build the prompt
	messages := ab.BuildRAGPrompt(query, limitedContext, conversationHistory, ab.config.IncludeCitations)

	// Calculate total tokens (simplified)
	for _, msg := range messages {
		metadata.TotalTokens += len(msg.Content) / 4 // Rough estimation
	}

	return messages, metadata
}

// PromptMetadata contains metadata about a generated prompt
type PromptMetadata struct {
	TotalTokens       int
	ContextTokens     int
	HistoryTokens     int
	DocumentsIncluded int
	TruncatedHistory  bool
	TruncatedContext  bool
}

// FormatSystemPromptWithRole creates a system prompt with a specific role
func FormatSystemPromptWithRole(role, additionalInstructions string) string {
	return fmt.Sprintf(`You are %s.

%s

Always maintain this role throughout the conversation and provide responses that are consistent with your expertise and perspective.`, role, additionalInstructions)
}

// CreateFocusedPrompt creates a prompt focused on a specific aspect
func CreateFocusedPrompt(focus, query string, context *RAGContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Focus: %s\n\n", focus))

	if context != nil && len(context.Documents) > 0 {
		sb.WriteString("Relevant Information:\n")
		for i, doc := range context.Documents {
			if i >= 3 { // Limit to top 3 for focused prompts
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", doc.Document.Content))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Query: %s\n\n", query))
	sb.WriteString(fmt.Sprintf("Please provide a response focused specifically on %s.", focus))

	return sb.String()
}
