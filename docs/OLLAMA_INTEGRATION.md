# Ollama Integration Guide

This guide describes the Ollama integration in the Connect LLM project, which provides the language model capabilities for the RAG-enabled chat service.

## Overview

The Ollama integration provides:

- Chat completion API for generating responses
- Streaming support for real-time response generation
- Model management capabilities
- Configurable model parameters for optimal performance

## Architecture

The Ollama service runs as a Docker container alongside Weaviate and the application server. The integration consists of:

1. **Ollama Service**: Runs in a Docker container with GPU support (optional)
2. **Go Client**: Located in `pkg/ollama/`, provides a clean API for interacting with Ollama
3. **Configuration**: Managed through environment variables and the config system

## Configuration

### Docker Compose

The Ollama service is configured in `docker-compose.yml`:

```yaml
ollama:
  image: ollama/ollama:latest
  ports:
    - "11434:11434"
  volumes:
    - ollama_data:/root/.ollama
  environment:
    OLLAMA_KEEP_ALIVE: "5m"
    OLLAMA_NUM_PARALLEL: "1"
  restart: unless-stopped
  deploy:
    resources:
      reservations:
        devices:
          - driver: nvidia
            count: 1
            capabilities: [gpu]
```

### Environment Variables

Configure the Ollama URL in your environment:

```bash
OLLAMA_URL=http://localhost:11434  # Default value
```

When running with Docker Compose, use the service name:

```bash
OLLAMA_URL=http://ollama:11434
```

## Client Usage

### Basic Chat Completion

```go
import "github.com/testsabirweb/connect_llm/pkg/ollama"

// Create client
client := ollama.NewClient("http://localhost:11434")

// Create chat request
req := ollama.ChatRequest{
    Model: "llama3:8b",
    Messages: []ollama.Message{
        {
            Role:    "system",
            Content: "You are a helpful assistant.",
        },
        {
            Role:    "user",
            Content: "Hello, how are you?",
        },
    },
    Options: &ollama.Options{
        Temperature: 0.7,
        NumPredict:  500,
    },
}

// Send request
resp, err := client.Chat(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Message.Content)
```

### Streaming Responses

```go
// Create streaming request
respChan, errChan := client.ChatStream(context.Background(), req)

// Process stream
for {
    select {
    case resp, ok := <-respChan:
        if !ok {
            return // Stream completed
        }
        fmt.Print(resp.Message.Content)
        if resp.Done {
            return
        }
    case err := <-errChan:
        if err != nil {
            log.Fatal(err)
        }
    }
}
```

### Model Management

```go
// List available models
models, err := client.ListModels(context.Background())
for _, model := range models {
    fmt.Printf("Model: %s, Size: %d\n", model.Name, model.Size)
}

// Pull a new model (if needed)
err = client.PullModel(context.Background(), "llama3:8b")
```

## Model Parameters

The following parameters can be configured in the `Options` struct:

- `Temperature` (0.0-1.0): Controls randomness in responses
- `NumPredict`: Maximum number of tokens to generate
- `TopK`: Limits token selection to top K tokens
- `TopP`: Nucleus sampling threshold
- `NumCtx`: Context window size

## Performance Optimization

### GPU Support

The Docker Compose configuration includes GPU support. If you have an NVIDIA GPU:

1. Install NVIDIA Container Toolkit
2. Ensure Docker can access the GPU
3. The container will automatically use GPU acceleration

### CPU Fallback

If no GPU is available, Ollama will automatically fall back to CPU inference. Consider:

- Using smaller models (e.g., llama3:7b instead of larger variants)
- Adjusting `OLLAMA_NUM_PARALLEL` to match CPU cores
- Reducing context window size for better performance

### Model Configuration

For optimal performance:

```go
options := &ollama.Options{
    Temperature: 0.7,      // Balance between creativity and consistency
    NumPredict:  500,      // Reasonable response length
    NumCtx:      4096,     // Standard context window
}
```

## Testing

Run the Ollama client tests:

```bash
go test -v ./pkg/ollama/...
```

Run the demo application:

```bash
go run examples/ollama/main.go
```

## Troubleshooting

### Connection Refused

If you get connection errors:

1. Check if Ollama is running: `docker ps | grep ollama`
2. Verify the port: `curl http://localhost:11434/api/tags`
3. Check Docker logs: `docker logs connect_llm-ollama-1`

### Model Not Found

If the llama3:8b model is not available:

1. Pull the model manually: `docker exec connect_llm-ollama-1 ollama pull llama3:8b`
2. Or use the client: `client.PullModel(ctx, "llama3:8b")`

### Performance Issues

For slow responses:

1. Check if GPU is being used: `docker exec connect_llm-ollama-1 nvidia-smi`
2. Monitor resource usage: `docker stats connect_llm-ollama-1`
3. Consider using a smaller model or adjusting parameters

## Integration with RAG

The Ollama client is designed to work seamlessly with the RAG-enabled chat service. The chat service will:

1. Retrieve relevant documents from Weaviate
2. Format them as context in the system message
3. Send the augmented prompt to Ollama
4. Stream the response back to the user

This integration ensures that responses are grounded in the indexed data while leveraging the language model's capabilities.
