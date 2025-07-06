package ollama

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:11434")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("expected baseURL to be http://localhost:11434, got %s", client.baseURL)
	}
}

func TestClientPing(t *testing.T) {
	// Skip if Ollama is not running
	client := NewClient("http://localhost:11434")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Ping(ctx)
	if err != nil {
		t.Skipf("Ollama server not available: %v", err)
	}
}

func TestListModels(t *testing.T) {
	client := NewClient("http://localhost:11434")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First check if server is available
	if err := client.Ping(ctx); err != nil {
		t.Skipf("Ollama server not available: %v", err)
	}

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("failed to list models: %v", err)
	}

	// Check if llama3:8b is available
	found := false
	for _, model := range models {
		if model.Name == "llama3:8b" {
			found = true
			break
		}
	}

	if !found {
		t.Log("llama3:8b model not found in available models")
	}
}

func TestChat(t *testing.T) {
	client := NewClient("http://localhost:11434")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First check if server is available
	if err := client.Ping(ctx); err != nil {
		t.Skipf("Ollama server not available: %v", err)
	}

	// Check if llama3:8b model is available
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Skipf("failed to list models: %v", err)
	}

	found := false
	for _, model := range models {
		if model.Name == "llama3:8b" {
			found = true
			break
		}
	}

	if !found {
		t.Skip("llama3:8b model not available")
	}

	// Test chat completion
	req := ChatRequest{
		Model: "llama3:8b",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant. Keep responses brief.",
			},
			{
				Role:    "user",
				Content: "Say hello in one word.",
			},
		},
		Options: &Options{
			Temperature: 0.1,
			NumPredict:  10,
		},
	}

	resp, err := client.Chat(ctx, req)
	if err != nil {
		t.Fatalf("failed to chat: %v", err)
	}

	if resp.Message.Role != "assistant" {
		t.Errorf("expected assistant role, got %s", resp.Message.Role)
	}

	if resp.Message.Content == "" {
		t.Error("expected non-empty response content")
	}

	t.Logf("Chat response: %s", resp.Message.Content)
}

func TestChatStream(t *testing.T) {
	client := NewClient("http://localhost:11434")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First check if server is available
	if err := client.Ping(ctx); err != nil {
		t.Skipf("Ollama server not available: %v", err)
	}

	// Check if llama3:8b model is available
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Skipf("failed to list models: %v", err)
	}

	found := false
	for _, model := range models {
		if model.Name == "llama3:8b" {
			found = true
			break
		}
	}

	if !found {
		t.Skip("llama3:8b model not available")
	}

	// Test streaming chat
	req := ChatRequest{
		Model: "llama3:8b",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Count from 1 to 5",
			},
		},
		Options: &Options{
			Temperature: 0.1,
			NumPredict:  50,
		},
	}

	respChan, errChan := client.ChatStream(ctx, req)

	var fullResponse string
	chunks := 0

	for {
		select {
		case resp, ok := <-respChan:
			if !ok {
				goto done
			}
			fullResponse += resp.Message.Content
			chunks++
			if resp.Done {
				goto done
			}
		case err := <-errChan:
			if err != nil {
				t.Fatalf("stream error: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("context timeout")
		}
	}

done:
	if chunks == 0 {
		t.Error("expected at least one chunk")
	}

	if fullResponse == "" {
		t.Error("expected non-empty response")
	}

	t.Logf("Received %d chunks, full response: %s", chunks, fullResponse)
}
