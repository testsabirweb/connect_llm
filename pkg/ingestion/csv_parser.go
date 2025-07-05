package ingestion

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/testsabirweb/connect_llm/pkg/models"
)

// ParserConfig contains configuration for the CSV parser
type ParserConfig struct {
	BatchSize       int  // Number of records to process in a batch
	SkipErrors      bool // Whether to skip records with errors
	ValidateRecords bool // Whether to validate records
}

// DefaultParserConfig returns default parser configuration
func DefaultParserConfig() ParserConfig {
	return ParserConfig{
		BatchSize:       100,
		SkipErrors:      true,
		ValidateRecords: true,
	}
}

// CSVParser handles parsing of Slack CSV export files
type CSVParser struct {
	config           ParserConfig
	totalRecords     int
	processedRecords int
	errorCount       int
	errors           []error
}

// NewCSVParser creates a new CSV parser instance
func NewCSVParser(config ...ParserConfig) *CSVParser {
	cfg := DefaultParserConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return &CSVParser{
		config: cfg,
		errors: make([]error, 0),
	}
}

// BatchCallback is called for each batch of messages
type BatchCallback func(messages []models.SlackMessage, batchNum int) error

// ProgressCallback is called to report progress
type ProgressCallback func(processed, total int, errors int)

// ParseFile parses a CSV file with batch processing and progress tracking
func (p *CSVParser) ParseFile(filename string, batchCallback BatchCallback, progressCallback ProgressCallback) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size for progress tracking
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	return p.ParseWithCallbacks(file, fileInfo.Size(), batchCallback, progressCallback)
}

// ParseWithCallbacks parses CSV data with batch processing
func (p *CSVParser) ParseWithCallbacks(r io.Reader, totalSize int64, batchCallback BatchCallback, progressCallback ProgressCallback) error {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true // Handle quotes in fields
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Map header columns
	columnMap := make(map[string]int)
	for i, col := range header {
		columnMap[strings.TrimSpace(col)] = i
	}

	// Validate required columns
	requiredColumns := []string{"text", "user", "channel_id", "ts", "type"}
	for _, col := range requiredColumns {
		if _, ok := columnMap[col]; !ok {
			return fmt.Errorf("required column %s not found in CSV", col)
		}
	}

	batch := make([]models.SlackMessage, 0, p.config.BatchSize)
	batchNum := 0
	p.totalRecords = 0
	p.processedRecords = 0
	p.errorCount = 0

	// Read records
	for {
		record, err := reader.Read()
		if err == io.EOF {
			// Process final batch
			if len(batch) > 0 {
				if err := batchCallback(batch, batchNum); err != nil {
					return fmt.Errorf("batch callback error: %w", err)
				}
			}
			break
		}
		if err != nil {
			if p.config.SkipErrors {
				p.recordError(fmt.Errorf("failed to read record %d: %w", p.totalRecords+1, err))
				p.totalRecords++
				continue
			}
			return fmt.Errorf("failed to read record: %w", err)
		}

		p.totalRecords++

		msg, err := p.parseRecord(record, columnMap)
		if err != nil {
			if p.config.SkipErrors {
				p.recordError(fmt.Errorf("failed to parse record %d: %w", p.totalRecords, err))
				continue
			}
			return fmt.Errorf("failed to parse record %d: %w", p.totalRecords, err)
		}

		// Validate record if configured
		if p.config.ValidateRecords {
			if err := p.validateMessage(msg); err != nil {
				if p.config.SkipErrors {
					p.recordError(fmt.Errorf("invalid record %d: %w", p.totalRecords, err))
					continue
				}
				return fmt.Errorf("invalid record %d: %w", p.totalRecords, err)
			}
		}

		batch = append(batch, msg)
		p.processedRecords++

		// Process batch when full
		if len(batch) >= p.config.BatchSize {
			if err := batchCallback(batch, batchNum); err != nil {
				return fmt.Errorf("batch callback error: %w", err)
			}
			batchNum++
			batch = make([]models.SlackMessage, 0, p.config.BatchSize)
		}

		// Report progress
		if progressCallback != nil && p.totalRecords%100 == 0 {
			progressCallback(p.processedRecords, p.totalRecords, p.errorCount)
		}
	}

	// Final progress report
	if progressCallback != nil {
		progressCallback(p.processedRecords, p.totalRecords, p.errorCount)
	}

	return nil
}

// Parse parses all messages at once (for smaller files)
func (p *CSVParser) Parse(r io.Reader) ([]models.SlackMessage, error) {
	var allMessages []models.SlackMessage

	err := p.ParseWithCallbacks(r, 0, func(messages []models.SlackMessage, batchNum int) error {
		allMessages = append(allMessages, messages...)
		return nil
	}, nil)

	if err != nil {
		return nil, err
	}

	return allMessages, nil
}

// parseRecord converts a CSV record to SlackMessage
func (p *CSVParser) parseRecord(record []string, columnMap map[string]int) (models.SlackMessage, error) {
	msg := models.SlackMessage{}

	// Helper function to get field value safely
	getField := func(fieldName string) string {
		if idx, ok := columnMap[fieldName]; ok && idx < len(record) {
			return strings.TrimSpace(record[idx])
		}
		return ""
	}

	// Parse core fields
	msg.Content = getField("text")
	msg.User = getField("user")
	msg.Channel = getField("channel_id")
	msg.Type = getField("type")
	msg.Subtype = getField("subtype")
	msg.ThreadTS = getField("thread_ts")

	// Use ts as message ID if client_msg_id is empty
	msg.MessageID = getField("client_msg_id")
	if msg.MessageID == "" {
		msg.MessageID = getField("ts")
	}

	// Parse timestamp
	tsStr := getField("ts")
	if tsStr != "" {
		// Slack timestamps are Unix timestamps with microseconds (e.g., "1599934232.150700")
		if ts, err := parseSlackTimestamp(tsStr); err == nil {
			msg.Timestamp = ts
		} else {
			return msg, fmt.Errorf("failed to parse timestamp %s: %w", tsStr, err)
		}
	}

	// Parse additional fields
	msg.ParentUserID = getField("parent_user_id")
	msg.BotID = getField("bot_id")
	msg.Reactions = getField("reactions")

	// Parse reply count
	if replyCountStr := getField("reply_count"); replyCountStr != "" {
		if count, err := strconv.Atoi(replyCountStr); err == nil {
			msg.ReplyCount = count
		}
	}

	// Parse reply users (JSON array string)
	if replyUsersStr := getField("reply_users"); replyUsersStr != "" {
		msg.ReplyUsers = parseJSONArrayString(replyUsersStr)
	}

	// Parse file IDs (JSON array string)
	if fileIDsStr := getField("file_ids"); fileIDsStr != "" {
		msg.FileIDs = parseJSONArrayString(fileIDsStr)
	}

	return msg, nil
}

// parseSlackTimestamp parses Slack's timestamp format
func parseSlackTimestamp(ts string) (time.Time, error) {
	// Try Unix timestamp with microseconds format first (e.g., "1599934232.150700")
	if strings.Contains(ts, ".") {
		parts := strings.Split(ts, ".")
		if len(parts) == 2 {
			seconds, err := strconv.ParseInt(parts[0], 10, 64)
			if err == nil {
				microseconds, err := strconv.ParseInt(parts[1], 10, 64)
				if err == nil {
					// Convert to nanoseconds
					nanos := seconds*1e9 + microseconds*1000
					return time.Unix(0, nanos), nil
				}
			}
		}
	} else {
		// Try parsing as Unix timestamp (seconds only)
		if seconds, err := strconv.ParseInt(ts, 10, 64); err == nil {
			return time.Unix(seconds, 0), nil
		}
	}

	// Try common datetime formats
	formats := []string{
		"2006-01-02 15:04:05",       // YYYY-MM-DD HH:MM:SS
		"2006-01-02T15:04:05Z",      // ISO 8601
		"2006-01-02T15:04:05-07:00", // ISO 8601 with timezone
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, ts); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid timestamp format: %s", ts)
}

// parseJSONArrayString parses a JSON array string like ["user1", "user2"]
func parseJSONArrayString(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" || s == "null" {
		return nil
	}

	// Remove brackets and quotes, then split
	s = strings.Trim(s, "[]")
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.ReplaceAll(s, `'`, "")

	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// validateMessage validates a SlackMessage
func (p *CSVParser) validateMessage(msg models.SlackMessage) error {
	// Skip system messages without user
	if msg.Subtype == "channel_join" || msg.Subtype == "channel_leave" {
		return nil
	}

	// Messages with files might have empty content - that's OK
	if msg.Content == "" && msg.Type == "message" && msg.Subtype == "" && len(msg.FileIDs) == 0 { //nolint:staticcheck // Intentionally empty - being lenient with empty messages
		// Only flag as error if there are no file attachments
		// In real Slack data, messages can be empty if they contain only files/attachments
		// For now, we'll be lenient and not treat this as an error
		// return fmt.Errorf("empty content for regular message")
	}

	if msg.User == "" && msg.BotID == "" {
		return fmt.Errorf("no user or bot ID")
	}

	if msg.Channel == "" {
		return fmt.Errorf("no channel ID")
	}

	if msg.Timestamp.IsZero() {
		return fmt.Errorf("invalid timestamp")
	}

	return nil
}

// recordError records a parsing error
func (p *CSVParser) recordError(err error) {
	p.errorCount++
	p.errors = append(p.errors, err)
}

// GetErrors returns all parsing errors
func (p *CSVParser) GetErrors() []error {
	return p.errors
}

// GetStats returns parsing statistics
func (p *CSVParser) GetStats() (total, processed, errors int) {
	return p.totalRecords, p.processedRecords, p.errorCount
}
