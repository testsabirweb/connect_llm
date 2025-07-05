# ConnectLLM - Internal Knowledge Search Platform

An intelligent, unified search and discovery platform for company data, similar to Glean. Built with Golang, Weaviate vector database, and Ollama for conversational AI.

## Project Overview

ConnectLLM addresses the critical problem of information silos in organizations by providing:
- **Semantic Search**: Natural language search across all indexed company data
- **Conversational AI**: Chat interface powered by Ollama's llama3:8b model
- **Unified Access**: Single interface for multiple data sources (starting with CSV, expanding to Slack, Jira, GitHub, Google Workspace)
- **Permission-Aware**: Respects source system permissions

## Tech Stack

- **Backend**: Golang
- **Vector Database**: Weaviate
- **LLM**: Ollama with llama3:8b model
- **Frontend**: React with TypeScript and Tailwind CSS
- **Infrastructure**: Docker Compose
- **Task Management**: Taskmaster AI with DeepSeek

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.21+
- Node.js 18+ (for frontend development)
- NVIDIA GPU (for Ollama) or CPU mode

### Quick Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd connect_llm
   ```

2. **Set up Ollama with llama3:8b**
   ```bash
   # Start Ollama container with GPU support
   docker run -d --name ollama \
     --gpus all -p 11434:11434 \
     -e OLLAMA_KEEP_ALIVE=-1 \
     -e OLLAMA_MAX_LOADED_MODELS=1 \
     -v ollama:/root/.ollama \
     ollama/ollama:latest

   # Pull and run llama3:8b model
   docker exec -it ollama ollama pull llama3:8b
   docker exec -it ollama ollama run llama3:8b
   ```

3. **Start all services**
   ```bash
   docker-compose up -d
   ```

## Task Management with Taskmaster

This project uses Taskmaster AI for task tracking and project management.

### API Configuration

Taskmaster is configured to use AI models through OpenRouter. 

✅ **Successfully Configured**:
- OpenRouter API key is set up in `.cursor/mcp.json`
- All models (main, research, fallback) are using **FREE** `deepseek/deepseek-chat-v3-0324:free` via OpenRouter
- AI-powered features like `parse-prd`, `expand`, and `research` are now available at no cost!

**Current Configuration**:
- Provider: OpenRouter
- Model: `deepseek/deepseek-chat-v3-0324:free` (DeepSeek V3 Free Tier)
- Cost: **$0/1M tokens** (FREE!)
- Context: 163,840 tokens
- Model Size: 671B total parameters, 37B activated per token

**About DeepSeek V3**:
According to [DeepSeek's official release](https://huggingface.co/unsloth/DeepSeek-V3-0324), this model features:
- Strong reasoning capabilities with significant benchmark improvements
- Enhanced front-end web development and coding abilities
- Improved Chinese and English writing proficiency
- Function calling support

**To switch back to paid models** (if needed):
```bash
# GPT-4o mini (low cost)
tm models --set-main openai/gpt-4o-mini

# DeepSeek V3 (paid version with priority access)
tm models --set-main deepseek/deepseek-chat-v3-0324
```

For more details, see `API_SETUP.md`.

### Viewing Tasks

```bash
# View all tasks
tm get-tasks

# View next task to work on
tm next

# View specific task details
tm get-task 1

# View tasks by status
tm get-tasks --status pending
```

### Working on Tasks

```bash
# Mark task as in-progress
tm set-status 1 in-progress

# Mark task as complete
tm set-status 1 done

# Update task with new information
tm update-task 1 --prompt "Add support for custom embeddings"
```

### Task Structure

The project is organized into 15 main tasks:

1. **Phase 1 - MVP (Tasks 1-6)**: Core search functionality with CSV data
2. **Phase 2 - AI Chat (Tasks 7-8)**: Ollama integration and chat interface
3. **Phase 3 - Frontend (Task 9)**: React-based UI
4. **Phase 4 - Enterprise (Tasks 10-15)**: Authentication, analytics, monitoring

## Data Ingestion

Initial data ingestion uses CSV files from the `slack/` directory with the following format:

```csv
message_id,timestamp,channel,user,content,thread_id,reactions
msg_001,2024-01-15T10:30:00Z,#general,user@company.com,"Team meeting notes...",thread_123,"👍:5,✅:3"
```

## API Endpoints

- `POST /api/v1/search` - Execute semantic search
- `GET /api/v1/documents/{id}` - Retrieve document details
- `POST /api/v1/chat` - Initiate chat session
- `WS /api/v1/chat/{session_id}` - WebSocket for real-time chat

## Development

### Project Structure
```
connect_llm/
├── .taskmaster/          # Taskmaster configuration and tasks
│   ├── docs/            # PRD and documentation
│   └── tasks/           # Task definitions
├── slack/               # CSV data files
├── cmd/                 # Application entrypoints
├── internal/            # Private application code
│   ├── api/            # REST API handlers
│   ├── chat/           # Chat service
│   ├── ingestion/      # Data ingestion pipeline
│   └── search/         # Search implementation
├── pkg/                 # Public packages
├── web/                 # Frontend React application
└── docker-compose.yml   # Service orchestration
```

### Building from Source

```bash
# Backend
go mod download
go build -o connect-llm cmd/server/main.go

# Frontend
cd web
npm install
npm run build
```

## Docker Deployment

The complete stack is defined in `docker-compose.yml`:

```yaml
version: '3.8'
services:
  weaviate:
    image: semitechnologies/weaviate:latest
    ports:
      - "8080:8080"
    volumes:
      - weaviate_data:/var/lib/weaviate

  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]

  search-api:
    build: ./search-service
    ports:
      - "8000:8000"
    depends_on:
      - weaviate
      - ollama

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
```

## Performance Targets

- Search latency: < 200ms (95th percentile)
- Ingestion rate: > 1000 documents/second
- Concurrent users: 1000+
- LLM response: < 2 seconds for first token

## Security Considerations

- JWT-based authentication
- Document-level permissions
- TLS for all communications
- Regular security audits

## Contributing

1. Check the next available task: `tm next`
2. Set task to in-progress: `tm set-status <task-id> in-progress`
3. Create a feature branch
4. Implement the task
5. Submit a pull request
6. Mark task as done: `tm set-status <task-id> done`

## License

[Your License Here]

## Support

For questions or issues:
1. Check existing tasks: `tm get-tasks`
2. Review the PRD: `.taskmaster/docs/prd.txt`
3. Open an issue in the repository 