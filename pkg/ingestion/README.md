# Ingestion Package

This package provides functionality for ingesting and parsing Slack data exports in CSV format.

## Components

### CSV Parser

The CSV parser handles reading and parsing Slack export CSV files with support for:

- Batch processing for large files
- Error handling and recovery
- Progress tracking
- Validation of records
- Support for all Slack message fields including threads, reactions, and file attachments

### Ingestion Service

The ingestion service orchestrates the complete data ingestion pipeline:

- **Concurrent Processing**: Processes messages in parallel using configurable worker pools
- **Batch Processing**: Handles large datasets efficiently with configurable batch sizes
- **Document Generation**: Converts Slack messages to searchable documents with embeddings
- **Vector Storage**: Stores processed documents in Weaviate for semantic search
- **Progress Tracking**: Provides detailed statistics and error reporting

## Usage

### Using the CSV Parser

```go
import "github.com/testsabirweb/connect_llm/pkg/ingestion"

// Create parser with custom configuration
parser := ingestion.NewCSVParser(ingestion.ParserConfig{
    BatchSize:       100,
    SkipErrors:      true,
    ValidateRecords: true,
})

// Parse a file
messages, err := parser.ParseFile("slack_export.csv",
    func(messages []ingestion.SlackMessage, batchNum int) error {
        // Process each batch
        fmt.Printf("Processing batch %d with %d messages\n", batchNum, len(messages))
        return nil
    },
    func(processed, total, errors int) {
        // Progress callback
        fmt.Printf("Progress: %d/%d (errors: %d)\n", processed, total, errors)
    },
)
```

### Using the Ingestion Service

```go
// Create service with dependencies
service := ingestion.NewService(
    vectorClient,      // Weaviate client
    documentProcessor, // Document processor with embeddings
    ingestion.ServiceConfig{
        BatchSize:        100,
        MaxConcurrency:   5,
        SkipEmptyContent: true,
    },
)

// Ingest a single file
stats, err := service.IngestFile(ctx, "path/to/file.csv")

// Ingest all CSV files in a directory
stats, err := service.IngestDirectory(ctx, "path/to/directory")

// Check results
summary := stats.GetSummary()
fmt.Printf("Processed: %v messages\n", summary["processed_messages"])
fmt.Printf("Stored: %v documents\n", summary["stored_documents"])
fmt.Printf("Duration: %.2f seconds\n", summary["duration_seconds"])
```

## Data Structure

The `SlackMessage` struct represents a parsed Slack message:

```go
type SlackMessage struct {
    MessageID    string    // Unique message identifier
    Timestamp    time.Time // Message timestamp
    Channel      string    // Channel ID
    User         string    // User ID
    Content      string    // Message content
    ThreadTS     string    // Thread timestamp (if part of thread)
    Type         string    // Message type
    Subtype      string    // Message subtype
    ReplyCount   int       // Number of replies
    ReplyUsers   []string  // Users who replied
    Reactions    string    // Reactions JSON
    ParentUserID string    // Parent message user (for threads)
    BotID        string    // Bot ID (if from bot)
    FileIDs      []string  // Attached file IDs
}
```

## Service Configuration

The ingestion service can be configured with:

- **BatchSize**: Number of messages to process in each batch (default: 100)
- **MaxConcurrency**: Maximum number of concurrent workers (default: 5)
- **SkipEmptyContent**: Whether to skip messages with no content (default: true)

## Error Handling

The service provides comprehensive error handling:

- Continues processing on individual message failures (configurable)
- Collects all errors for reporting
- Provides detailed statistics on success/failure rates
- Supports graceful shutdown via context cancellation

## Performance Considerations

- Use larger batch sizes for better throughput with stable data
- Increase concurrency for CPU-bound operations (embeddings)
- Monitor memory usage with very large files
- Consider chunking very long messages for better search results

## CSV File Format

The parser expects CSV files with the following columns:

- `client_msg_id` or `ts`: Message identifier
- `text`: Message content
- `user`: User ID
- `channel_id`: Channel ID
- `type`: Message type
- `thread_ts`: Thread timestamp (optional)
- `reply_count`: Number of replies (optional)
- `reply_users`: JSON array of reply user IDs (optional)
- `reactions`: Reactions data (optional)
- `file_ids`: JSON array of file IDs (optional)
- `subtype`: Message subtype (optional)
- `bot_id`: Bot ID for bot messages (optional)
- `parent_user_id`: Parent message user ID (optional)

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./pkg/ingestion/...

# Run with coverage
go test -cover ./pkg/ingestion/...

# Run specific test
go test -run TestCSVParser ./pkg/ingestion/...
```

## Thread Safety

- The CSV parser is safe for concurrent use
- The ingestion service handles concurrent processing internally
- Statistics are updated atomically with mutex protection
