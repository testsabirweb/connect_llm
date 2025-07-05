# ConnectLLM Examples

This directory contains example code demonstrating how to use various components of the ConnectLLM system.

## Examples

### Ingestion Example (`ingestion_example.go`)

Demonstrates how to use the ingestion service programmatically:

- Setting up the ingestion service with custom configuration
- Ingesting single files
- Ingesting entire directories
- Custom batch processing with fine-grained control
- Error handling and progress tracking

**Usage:**

```bash
# Make sure services are running
make docker-up
make setup-weaviate

# Run the example
go run examples/ingestion_example.go
```

## Coming Soon

- Search API examples (Task 6)
- Chat interface examples
- Real-time processing examples
- Custom embeddings examples

## Tips

1. **Configuration**: All examples use the standard configuration from environment variables or `.env` file.

2. **Error Handling**: Examples demonstrate proper error handling patterns that should be used in production code.

3. **Performance**: Examples show how to configure batch sizes and concurrency for optimal performance.

4. **Custom Processing**: Examples demonstrate how to extend the system with custom processors and adapters.

## Contributing

Feel free to add more examples demonstrating different use cases or features!
