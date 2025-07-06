package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/testsabirweb/connect_llm/pkg/ollama"
)

func main() {
	// Create Ollama client
	client := ollama.NewClient("http://localhost:11434")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test 1: Ping the server
	fmt.Println("1. Testing connection to Ollama...")
	if err := client.Ping(ctx); err != nil {
		log.Fatalf("Failed to connect to Ollama: %v", err)
	}
	fmt.Println("✓ Successfully connected to Ollama")

	// Test 2: List available models
	fmt.Println("\n2. Listing available models...")
	models, err := client.ListModels(ctx)
	if err != nil {
		log.Fatalf("Failed to list models: %v", err)
	}

	fmt.Printf("✓ Found %d models:\n", len(models))
	for _, model := range models {
		fmt.Printf("  - %s (size: %.2f GB)\n", model.Name, float64(model.Size)/(1024*1024*1024))
	}

	// Test 3: Simple chat completion
	fmt.Println("\n3. Testing chat completion...")
	chatReq := ollama.ChatRequest{
		Model: "llama3:8b",
		Messages: []ollama.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant. Keep responses brief and to the point.",
			},
			{
				Role:    "user",
				Content: "What is the capital of France? Answer in one word.",
			},
		},
		Options: &ollama.Options{
			Temperature: 0.1,
			NumPredict:  20,
		},
	}

	resp, err := client.Chat(ctx, chatReq)
	if err != nil {
		log.Fatalf("Failed to chat: %v", err)
	}
	fmt.Printf("✓ Response: %s\n", resp.Message.Content)

	// Test 4: Streaming chat
	fmt.Println("\n4. Testing streaming chat...")
	streamReq := ollama.ChatRequest{
		Model: "llama3:8b",
		Messages: []ollama.Message{
			{
				Role:    "user",
				Content: "Count from 1 to 5, one number per line.",
			},
		},
		Options: &ollama.Options{
			Temperature: 0.1,
			NumPredict:  50,
		},
	}

	fmt.Print("✓ Streaming response: ")
	respChan, errChan := client.ChatStream(ctx, streamReq)

	for {
		select {
		case resp, ok := <-respChan:
			if !ok {
				fmt.Println("\n✓ Stream completed")
				return
			}
			fmt.Print(resp.Message.Content)
			if resp.Done {
				fmt.Println("\n✓ Stream completed")
				return
			}
		case err := <-errChan:
			if err != nil {
				log.Fatalf("\nStream error: %v", err)
			}
		case <-ctx.Done():
			log.Fatal("\nContext timeout")
		}
	}
}
