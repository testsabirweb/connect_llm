.PHONY: build run test docker-up docker-down setup-weaviate clean help

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

## run: Run the application
run: build
	./bin/server

## test: Run tests
test:
	go test -v ./...

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
