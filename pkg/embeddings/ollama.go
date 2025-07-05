package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaEmbedder handles embedding generation using Ollama
type OllamaEmbedder struct {
	client  *http.Client
	baseURL string
	model   string
}

// NewOllamaEmbedder creates a new Ollama embedder
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: baseURL,
		model:   model,
	}
}

// EmbedRequest represents the request to Ollama embed API
type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbedResponse represents the response from Ollama embed API
type EmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// GenerateEmbedding generates an embedding for the given text
func (e *OllamaEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	req := EmbedRequest{
		Model: e.model,
		Input: text,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embed", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embedResp.Embeddings) == 0 || len(embedResp.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embedResp.Embeddings[0], nil
}

// GenerateEmbeddings generates embeddings for multiple texts
func (e *OllamaEmbedder) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		embedding, err := e.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// GetEmbeddingDimension returns the dimension of embeddings for the model
func (e *OllamaEmbedder) GetEmbeddingDimension(ctx context.Context) (int, error) {
	// Generate a test embedding to get dimension
	embedding, err := e.GenerateEmbedding(ctx, "test")
	if err != nil {
		return 0, fmt.Errorf("failed to get embedding dimension: %w", err)
	}
	return len(embedding), nil
}
