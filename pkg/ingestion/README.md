# Ingestion Package

The ingestion package provides functionality for parsing and processing Slack export data from CSV files.

## CSV Parser

The CSV parser is designed to handle Slack message exports with the following features:

### Features

- **Batch Processing**: Process large CSV files in configurable batches to manage memory usage
- **Progress Tracking**: Real-time progress updates during parsing
- **Error Handling**: Configurable error handling with the ability to skip invalid records
- **Validation**: Built-in message validation with lenient rules for real-world data
- **Rich Data Support**: Handles thread IDs, reactions, reply counts, file attachments, and more

### Usage

```go
import "github.com/testsabirweb/connect_llm/pkg/ingestion"

// Create a new parser with default configuration
parser := ingestion.NewCSVParser()

// Or create with custom configuration
parser := ingestion.NewCSVParser(ingestion.ParserConfig{
    BatchSize:       100,  // Process 100 records per batch
    SkipErrors:      true, // Continue on errors
    ValidateRecords: true, // Validate message data
})

// Parse a CSV file with batch processing
err := parser.ParseFile("slack/messages.csv",
    func(messages []ingestion.SlackMessage, batchNum int) error {
        // Process each batch of messages
        fmt.Printf("Processing batch %d with %d messages\n", batchNum, len(messages))
        // ... save to database, send to API, etc.
        return nil
    },
    func(processed, total, errors int) {
        // Progress callback
        fmt.Printf("Progress: %d/%d messages, %d errors\n", processed, total, errors)
    },
)

// Get parsing statistics
total, processed, errorCount := parser.GetStats()
```

### SlackMessage Structure

```go
type SlackMessage struct {
    MessageID    string    // Unique message identifier
    Timestamp    time.Time // Message timestamp
    Channel      string    // Channel ID
    User         string    // User ID
    Content      string    // Message content
    ThreadTS     string    // Thread timestamp (for threaded messages)
    Type         string    // Message type
    Subtype      string    // Message subtype (e.g., channel_join)
    ReplyCount   int       // Number of replies
    ReplyUsers   []string  // Users who replied
    Reactions    string    // Reactions JSON
    ParentUserID string    // Parent user for threads
    BotID        string    // Bot ID (for bot messages)
    FileIDs      []string  // Attached file IDs
}
```

### CSV Format

The parser expects CSV files with the following required columns:

- `text`: Message content
- `user`: User ID
- `channel_id`: Channel ID
- `ts`: Timestamp
- `type`: Message type

Optional columns include:

- `thread_ts`: Thread timestamp
- `subtype`: Message subtype
- `bot_id`: Bot identifier
- `reply_count`: Number of replies
- `reply_users`: JSON array of reply user IDs
- `reactions`: Reactions data
- `file_ids`: JSON array of file IDs

### Timestamp Formats

The parser supports multiple timestamp formats:

- Unix timestamp with microseconds: `1599934232.150700`
- Unix timestamp (seconds): `1599934232`
- Human-readable: `2020-09-12 18:10:32`
- ISO 8601: `2020-09-12T18:10:32Z`

### Error Handling

When `SkipErrors` is enabled, the parser will:

- Continue processing on invalid records
- Track all errors for later review
- Report error count in statistics

Access errors with:

```go
errors := parser.GetErrors()
for _, err := range errors {
    fmt.Printf("Error: %v\n", err)
}
```

### Performance Considerations

- Default batch size is 100 records
- Larger batch sizes use more memory but may be faster
- Progress callbacks are called every 100 records by default
- For very large files (millions of records), consider using smaller batch sizes

### Testing

Run tests with:

```bash
go test ./pkg/ingestion -v
```

Test with actual Slack data:

```bash
go run cmd/test_csv_parser.go slack/messages.csv
```
