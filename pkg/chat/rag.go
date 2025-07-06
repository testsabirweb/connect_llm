package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/testsabirweb/connect_llm/pkg/embeddings"
	"github.com/testsabirweb/connect_llm/pkg/vector"
)

// RAGConfig holds configuration for the RAG system
type RAGConfig struct {
	MaxDocuments    int     // Maximum number of documents to retrieve
	MinScore        float64 // Minimum relevance score threshold
	MaxTokens       int     // Maximum tokens for context
	ChunkSize       int     // Approximate size of each chunk in tokens
	IncludeMetadata bool    // Whether to include document metadata
	DiversityFactor float64 // Factor for result diversity (0-1)
}

// DefaultRAGConfig returns default RAG configuration
func DefaultRAGConfig() RAGConfig {
	return RAGConfig{
		MaxDocuments:    10,
		MinScore:        0.5,
		MaxTokens:       4000,
		ChunkSize:       150, // Approximate words per chunk
		IncludeMetadata: true,
		DiversityFactor: 0.3,
	}
}

// RAGRetriever handles document retrieval for RAG
type RAGRetriever struct {
	vectorClient vector.Client
	embedder     *embeddings.OllamaEmbedder
	config       RAGConfig
}

// NewRAGRetriever creates a new RAG retriever
func NewRAGRetriever(vectorClient vector.Client, embedder *embeddings.OllamaEmbedder, config ...RAGConfig) *RAGRetriever {
	cfg := DefaultRAGConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return &RAGRetriever{
		vectorClient: vectorClient,
		embedder:     embedder,
		config:       cfg,
	}
}

// RetrievalResult represents a retrieved document with relevance information
type RetrievalResult struct {
	Document   vector.Document
	Score      float64
	Relevance  string // high, medium, low
	TokenCount int
}

// RAGContext represents the context built from retrieved documents
type RAGContext struct {
	Query       string
	Documents   []RetrievalResult
	TotalTokens int
	Metadata    map[string]interface{}
}

// RetrieveContext retrieves relevant documents for a query
func (r *RAGRetriever) RetrieveContext(ctx context.Context, query string, filters ...map[string]interface{}) (*RAGContext, error) {
	// Generate embeddings for the query
	queryEmbedding, err := r.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Prepare search options
	searchOpts := vector.SearchOptions{
		Query: queryEmbedding,
		Limit: r.config.MaxDocuments * 2, // Get more to filter later
	}

	// Add filters if provided
	if len(filters) > 0 {
		searchOpts.Filters = filters[0]
	}

	// Search for relevant documents
	documents, err := r.vectorClient.SearchWithOptions(ctx, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	// Process and rank results
	results := r.processResults(documents, query)

	// Apply diversity if configured
	if r.config.DiversityFactor > 0 {
		results = r.applyDiversity(results)
	}

	// Build context within token limits
	context := r.buildContext(query, results)

	return context, nil
}

// processResults processes and ranks search results
func (r *RAGRetriever) processResults(documents []vector.Document, query string) []RetrievalResult {
	results := make([]RetrievalResult, 0, len(documents))

	for _, doc := range documents {
		// Calculate relevance score (this is a simplified version)
		score := r.calculateRelevance(doc, query)

		if score < r.config.MinScore {
			continue
		}

		// Estimate token count
		tokenCount := r.estimateTokens(doc.Content)

		// Determine relevance level
		relevance := "low"
		if score > 0.8 {
			relevance = "high"
		} else if score > 0.65 {
			relevance = "medium"
		}

		results = append(results, RetrievalResult{
			Document:   doc,
			Score:      score,
			Relevance:  relevance,
			TokenCount: tokenCount,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// calculateRelevance calculates a relevance score for a document
func (r *RAGRetriever) calculateRelevance(doc vector.Document, query string) float64 {
	// This is a simplified relevance calculation
	// In production, you might use more sophisticated methods

	queryLower := strings.ToLower(query)
	contentLower := strings.ToLower(doc.Content)

	// Base score from vector similarity
	// TODO: Get actual similarity score from Weaviate search results
	// For now, use a default base score
	baseScore := 0.7

	// Boost score if query terms appear in content
	queryTerms := strings.Fields(queryLower)
	matchCount := 0
	for _, term := range queryTerms {
		if strings.Contains(contentLower, term) {
			matchCount++
		}
	}

	termBoost := float64(matchCount) / float64(len(queryTerms)) * 0.3

	// Boost for metadata matches
	metadataBoost := 0.0
	if doc.Metadata.Author != "" && strings.Contains(queryLower, strings.ToLower(doc.Metadata.Author)) {
		metadataBoost += 0.1
	}

	finalScore := baseScore + termBoost + metadataBoost
	if finalScore > 1.0 {
		finalScore = 1.0
	}

	return finalScore
}

// applyDiversity ensures diverse results
func (r *RAGRetriever) applyDiversity(results []RetrievalResult) []RetrievalResult {
	if len(results) <= r.config.MaxDocuments {
		return results
	}

	// Simple diversity: take top results and then sample from different sources
	diverse := make([]RetrievalResult, 0, r.config.MaxDocuments)
	seen := make(map[string]int)

	// First, take the top results
	topCount := int(float64(r.config.MaxDocuments) * (1 - r.config.DiversityFactor))
	for i := 0; i < topCount && i < len(results); i++ {
		diverse = append(diverse, results[i])
		source := results[i].Document.Source
		seen[source]++
	}

	// Then, add diverse results
	for _, result := range results[topCount:] {
		if len(diverse) >= r.config.MaxDocuments {
			break
		}

		source := result.Document.Source
		if seen[source] < 2 { // Limit per source
			diverse = append(diverse, result)
			seen[source]++
		}
	}

	return diverse
}

// buildContext builds the final RAG context
func (r *RAGRetriever) buildContext(query string, results []RetrievalResult) *RAGContext {
	context := &RAGContext{
		Query:     query,
		Documents: make([]RetrievalResult, 0),
		Metadata:  make(map[string]interface{}),
	}

	totalTokens := 0
	includedDocs := 0

	for _, result := range results {
		if totalTokens+result.TokenCount > r.config.MaxTokens {
			// Check if we can fit a truncated version
			remainingTokens := r.config.MaxTokens - totalTokens
			if remainingTokens > 100 { // Minimum useful chunk
				// Truncate the document
				truncated := r.truncateDocument(result.Document, remainingTokens)
				result.Document.Content = truncated
				result.TokenCount = r.estimateTokens(truncated)
				context.Documents = append(context.Documents, result)
				totalTokens += result.TokenCount
				includedDocs++
			}
			break
		}

		context.Documents = append(context.Documents, result)
		totalTokens += result.TokenCount
		includedDocs++
	}

	context.TotalTokens = totalTokens
	context.Metadata["total_retrieved"] = len(results)
	context.Metadata["included_documents"] = includedDocs
	context.Metadata["truncated"] = includedDocs < len(results)

	return context
}

// estimateTokens estimates the number of tokens in a text
func (r *RAGRetriever) estimateTokens(text string) int {
	// Simple estimation: ~4 characters per token
	return len(text) / 4
}

// truncateDocument truncates a document to fit within token limit
func (r *RAGRetriever) truncateDocument(doc vector.Document, maxTokens int) string {
	maxChars := maxTokens * 4 // Rough estimation
	if len(doc.Content) <= maxChars {
		return doc.Content
	}

	// Try to truncate at a sentence boundary
	truncated := doc.Content[:maxChars]
	lastPeriod := strings.LastIndex(truncated, ".")
	if lastPeriod > maxChars/2 {
		truncated = truncated[:lastPeriod+1]
	}

	return truncated + " [truncated]"
}

// FormatContextForPrompt formats the RAG context for LLM prompt
func (r *RAGRetriever) FormatContextForPrompt(context *RAGContext) string {
	var sb strings.Builder

	sb.WriteString("Based on the following relevant documents:\n\n")

	for i, result := range context.Documents {
		sb.WriteString(fmt.Sprintf("--- Document %d (Relevance: %s, Score: %.2f) ---\n",
			i+1, result.Relevance, result.Score))

		if r.config.IncludeMetadata && result.Document.Metadata.Title != "" {
			sb.WriteString(fmt.Sprintf("Title: %s\n", result.Document.Metadata.Title))
		}

		if r.config.IncludeMetadata && result.Document.Metadata.Author != "" {
			sb.WriteString(fmt.Sprintf("Author: %s\n", result.Document.Metadata.Author))
		}

		sb.WriteString(fmt.Sprintf("Content: %s\n\n", result.Document.Content))
	}

	sb.WriteString(fmt.Sprintf("Query: %s\n", context.Query))

	return sb.String()
}

// GetCitations extracts citations from the context
func (r *RAGRetriever) GetCitations(context *RAGContext) []Citation {
	citations := make([]Citation, 0, len(context.Documents))

	for _, result := range context.Documents {
		citation := Citation{
			DocumentID: result.Document.ID,
			Content:    result.Document.Content,
			Score:      result.Score,
			Metadata: map[string]interface{}{
				"title":     result.Document.Metadata.Title,
				"author":    result.Document.Metadata.Author,
				"source":    result.Document.Source,
				"relevance": result.Relevance,
			},
		}
		citations = append(citations, citation)
	}

	return citations
}
