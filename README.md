# ConnectLLM - Slack Data Search & Chat System

A Golang-based semantic search and chat system for Slack data using Weaviate vector database and Ollama LLM.

## Architecture

- **Backend**: Go (Golang)
- **Vector Database**: Weaviate
- **LLM**: Ollama with llama3:8b model
- **Frontend**: React (to be implemented)

## Project Structure

```text
connect_llm/
├── cmd/
│   └── server/         # Application entry point
├── pkg/
│   ├── api/           # HTTP API server and handlers
│   ├── ingestion/     # CSV data parsing and ingestion
│   └── vector/        # Vector database operations
├── internal/          # Private application code
│   ├── config/        # Configuration management
│   ├── models/        # Data models
│   └── utils/         # Utility functions
├── slack/             # Slack export data (CSV files)
├── docker-compose.yml # Docker services configuration
├── Dockerfile         # Application container
└── go.mod            # Go module definition
```

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- (Optional) NVIDIA GPU for Ollama acceleration

## Getting Started

### 1. Clone the repository

```bash
git clone https://github.com/testsabirweb/connect_llm.git
cd connect_llm
```

### 2. Set up environment variables

```bash
# Copy the example environment file
cp env-example .env

# Edit .env with your configuration (optional)
# Default values work for local development
```

### 3. Start the infrastructure

```bash
# Start Weaviate and Ollama services
make docker-up

# Or manually:
docker compose up -d weaviate ollama
```

### 4. Initialize Weaviate schema

```bash
# Set up the Document schema in Weaviate
make setup-weaviate

# Or with testing:
make setup-weaviate-test
```

### 5. Pull the Ollama model

```bash
# Pull the llama3:8b model
docker exec -it connect_llm-ollama-1 ollama pull llama3:8b
```

### 6. Build and run the application

```bash
# Using make (recommended)
make run

# Or manually:
go build -o bin/server cmd/server/main.go
./bin/server
```

### Quick start (all-in-one)

```bash
# Start everything with one command
make dev
```

## API Endpoints

- `GET /health` - Health check endpoint with Weaviate status

  ```json
  {
    "status": "healthy",
    "service": "connect-llm",
    "checks": {
      "weaviate": {
        "healthy": true,
        "error": ""
      }
    }
  }
  ```

- `GET /api/v1/search` - Search endpoint (to be implemented)
- `POST /api/v1/ingest` - Data ingestion endpoint (to be implemented)

## Environment Variables

- `PORT` - Server port (default: 8080)
- `WEAVIATE_URL` - Weaviate URL (default: <http://localhost:8000>)
- `OLLAMA_URL` - Ollama URL (default: <http://localhost:11434>)

## Development

### Security: Pre-commit Hooks

This project uses pre-commit hooks to prevent security issues and maintain code quality. To set up:

```bash
# Run the setup script
./scripts/setup-pre-commit.sh

# Or manually install pre-commit
pip install pre-commit
pre-commit install
```

The hooks will automatically:

- 🔒 **Prevent secrets** from being committed (API keys, passwords, etc.)
- 🐹 **Format and lint Go code** before commits
- 📝 **Validate** YAML, JSON, and other config files
- 🧹 **Clean up** trailing whitespace and file endings

To run hooks manually:

```bash
pre-commit run --all-files
```

To bypass hooks in emergencies (use sparingly):

```bash
git commit --no-verify
```

### Running tests

```bash
go test ./...
```

### Adding dependencies

```bash
go get <package>
go mod tidy
```

## Services

### Weaviate

- URL: <http://localhost:8000>
- gRPC: localhost:50051

### Ollama

- URL: <http://localhost:11434>

## Next Steps

1. ~~Configure Weaviate schema for document storage (Task 2)~~ ✅
2. ~~Implement CSV data parser for Slack export (Task 3)~~ ✅
3. ~~Build document processing pipeline (Task 4)~~ ✅
4. Create data ingestion service (Task 5)
5. Implement search API endpoints (Task 6)

## License

[To be added]
