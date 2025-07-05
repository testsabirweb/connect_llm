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
│   ├── server/         # Application entry point
│   ├── ingest/         # Data ingestion CLI tool
│   └── weaviate-setup/ # Weaviate schema setup
├── pkg/
│   ├── api/           # HTTP API server and handlers
│   ├── ingestion/     # CSV data parsing and ingestion service
│   ├── processing/    # Document processing and chunking
│   ├── embeddings/    # Ollama embedding generation
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

## Data Ingestion

The system includes a powerful data ingestion service that processes Slack CSV exports, generates embeddings, and stores them in Weaviate.

### Using the CLI Tool

```bash
# Build the ingestion tool
make build-ingest

# Ingest a single CSV file
make ingest INPUT=slack/channel_general.csv

# Ingest all CSV files in a directory
make ingest INPUT=slack/

# Ingest with custom settings
make ingest INPUT=slack/ ARGS='-batch-size 200 -concurrency 10'
```

### Using the API Endpoint

```bash
# Ingest a single file via API
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "type": "file",
    "path": "slack/channel_general.csv"
  }'

# Ingest a directory via API
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "type": "directory",
    "path": "slack/"
  }'
```

### Ingestion Options

The ingestion tool supports several configuration options:

- `--batch-size`: Number of messages to process in each batch (default: 100)
- `--concurrency`: Maximum number of concurrent workers (default: 5)
- `--chunk-size`: Maximum chunk size in words (default: 500)
- `--chunk-overlap`: Chunk overlap in words (default: 50)
- `--skip-empty`: Skip messages with empty content (default: true)
- `--embedding-model`: Ollama model to use for embeddings (default: llama3:8b)

### Quick Ingestion

For a complete setup and ingestion in one command:

```bash
# This will start services, setup schema, and optionally ingest data
make ingest-quick INPUT=slack/
```

## Search API

ConnectLLM provides a powerful semantic search API that allows you to search through ingested documents using natural language queries.

### Search Endpoints

The search functionality is available at `/api/v1/search` and supports both GET and POST methods.

#### Simple Search (GET)

```bash
# Basic search
curl "http://localhost:8080/api/v1/search?q=database%20migration"

# Search with pagination
curl "http://localhost:8080/api/v1/search?q=security&limit=20&offset=40"
```

#### Advanced Search (POST)

```bash
# Search with filters
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "authentication",
    "limit": 10,
    "filters": {
      "source": "slack",
      "tags": ["security", "auth"],
      "dateFrom": "2023-01-01T00:00:00Z"
    }
  }'
```

### Search Features

- **Semantic Search**: Uses vector embeddings to find conceptually related documents
- **Metadata Filtering**: Filter by source, author, tags, date range, and permissions
- **Pagination**: Support for limit/offset based pagination
- **Relevance Scoring**: Results are ranked by semantic similarity

### API Documentation

For detailed API documentation, see [docs/api.md](docs/api.md).

### Examples

Run the example script to see the search API in action:

```bash
./examples/search_example.sh
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

- `POST /api/v1/ingest` - Data ingestion endpoint

  Request body:

  ```json
  {
    "type": "file|directory",
    "path": "/path/to/data",
    "batch_size": 100  // optional
  }
  ```

  Response:

  ```json
  {
    "success": true,
    "stats": {
      "total_messages": 1000,
      "processed_messages": 950,
      "skipped_messages": 30,
      "failed_messages": 20,
      "total_documents": 1200,
      "stored_documents": 1180,
      "failed_documents": 20,
      "error_count": 20,
      "duration_seconds": 45.2,
      "messages_per_second": 21.0
    },
    "errors": ["error1", "error2"]
  }
  ```

- `GET /api/v1/search` - Search endpoint (to be implemented)

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
# Run unit tests (default)
make test

# Run tests with coverage
make test-coverage

# Run integration tests (requires Weaviate and Ollama running)
make docker-up  # Start services first
make test-integration

# Or manually:
INTEGRATION_TEST=true go test -v ./...
```

**Note**: Integration tests require Weaviate to be running on `localhost:8000`. They are skipped by default when running `make test` to allow for faster CI/CD pipelines.

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
4. ~~Create data ingestion service (Task 5)~~ ✅
5. ~~Implement search API endpoints (Task 6)~~ ✅
6. Set up Ollama integration (Task 7)
7. Build chat service backend (Task 8)

## License

[To be added]
