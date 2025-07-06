#!/bin/bash
# Initialize Ollama with required models

set -e

echo "Waiting for Ollama to be ready..."

# Wait for Ollama service to be available
max_attempts=30
attempt=0

while [ $attempt -lt $max_attempts ]; do
    if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
        echo "Ollama is ready!"
        break
    fi
    echo "Waiting for Ollama to start... (attempt $((attempt+1))/$max_attempts)"
    sleep 2
    attempt=$((attempt+1))
done

if [ $attempt -eq $max_attempts ]; then
    echo "Error: Ollama failed to start within timeout"
    exit 1
fi

# Check if llama3:8b model is already available
echo "Checking for llama3:8b model..."
if curl -s http://localhost:11434/api/tags | grep -q '"name":"llama3:8b"'; then
    echo "Model llama3:8b is already available"
else
    echo "Pulling llama3:8b model..."
    # Pull the model using Ollama API
    curl -X POST http://localhost:11434/api/pull \
        -H "Content-Type: application/json" \
        -d '{"name": "llama3:8b"}' \
        --no-buffer 2>/dev/null | while IFS= read -r line; do
        # Parse and display progress
        if echo "$line" | grep -q '"status"'; then
            status=$(echo "$line" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
            echo "Status: $status"
        fi
        if echo "$line" | grep -q '"completed"'; then
            completed=$(echo "$line" | grep -o '"completed":[0-9]*' | cut -d':' -f2)
            total=$(echo "$line" | grep -o '"total":[0-9]*' | cut -d':' -f2)
            if [ -n "$completed" ] && [ -n "$total" ] && [ "$total" -gt 0 ]; then
                percent=$((completed * 100 / total))
                echo "Progress: $percent% ($completed/$total)"
            fi
        fi
    done
    echo "Model llama3:8b pulled successfully!"
fi

echo "Ollama initialization complete!"
