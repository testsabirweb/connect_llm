.PHONY: build run test docker-up docker-down setup-weaviate clean help ingest build-ingest test-coverage ingest-quick

# Default target
default: help

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the application
build:
	go build -o bin/server cmd/server/main.go
	go build -o bin/weaviate-setup cmd/weaviate-setup/main.go

## build-ingest: Build the ingestion tool
build-ingest:
	go build -o bin/ingest cmd/ingest/main.go

## run: Run the application
run: build
	./bin/server

## ingest: Run data ingestion (use INPUT=path to specify data source)
ingest: build-ingest
	@if [ -z "$(INPUT)" ]; then \
		echo "Usage: make ingest INPUT=<path> [ARGS='additional arguments']"; \
		echo "Example: make ingest INPUT=slack/"; \
		echo "Example: make ingest INPUT=slack/channel.csv ARGS='-batch-size 200'"; \
		exit 1; \
	fi
	./bin/ingest -input $(INPUT) $(ARGS)

## test: Run tests
test:
	go test -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-integration: Run integration tests (requires services)
test-integration:
	go test -v ./... -run Integration

## docker-up: Start Docker services (Weaviate and Ollama)
docker-up:
	docker compose up -d weaviate ollama
	@echo "Waiting for services to be ready..."
	@sleep 10

## docker-down: Stop Docker services
docker-down:
	docker compose down

## setup-weaviate: Initialize Weaviate schema
setup-weaviate: build
	./bin/weaviate-setup

## setup-weaviate-test: Initialize Weaviate schema and run tests
setup-weaviate-test: build
	./bin/weaviate-setup test

## dev: Start development environment (Docker + setup + run)
dev: docker-up setup-weaviate run

## ingest-quick: Quick start for data ingestion (starts services, sets up schema, ingests data)
ingest-quick:
	@echo "Starting services..."
	@make docker-up
	@echo "Setting up Weaviate schema..."
	@make setup-weaviate
	@echo "Pulling Ollama model (this may take a while)..."
	@docker exec connect_llm-ollama-1 ollama pull llama3:8b || true
	@echo "Ready for ingestion!"
	@if [ -z "$(INPUT)" ]; then \
		echo ""; \
		echo "Now run: make ingest INPUT=<path>"; \
		echo "Example: make ingest INPUT=slack/"; \
	else \
		make ingest INPUT=$(INPUT) $(ARGS); \
	fi

## clean: Clean build artifacts
clean:
	rm -rf bin/
	go clean -cache

## lint: Run linters
lint:
	$(HOME)/go/bin/golangci-lint run

## fmt: Format code
fmt:
	go fmt ./...

## mod-tidy: Tidy go modules
mod-tidy:
	go mod tidy

## pre-commit: Run pre-commit hooks on all files
pre-commit:
	pre-commit run --all-files
