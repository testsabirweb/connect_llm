package ingestion

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"time"
)

// SlackMessage represents a message from Slack export
type SlackMessage struct {
	MessageID string
	Timestamp time.Time
	Channel   string
	User      string
	Content   string
	ThreadID  string
	Reactions string
}

// CSVParser handles parsing of Slack CSV export files
type CSVParser struct {
	// Add configuration fields if needed
}

// NewCSVParser creates a new CSV parser instance
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// ParseFile parses a CSV file and returns slice of SlackMessages
func (p *CSVParser) ParseFile(filename string) ([]SlackMessage, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return p.Parse(file)
}

// Parse parses CSV data from a reader
func (p *CSVParser) Parse(r io.Reader) ([]SlackMessage, error) {
	reader := csv.NewReader(r)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Map header columns
	columnMap := make(map[string]int)
	for i, col := range header {
		columnMap[col] = i
	}

	var messages []SlackMessage

	// Read records
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read record: %w", err)
		}

		msg, err := p.parseRecord(record, columnMap)
		if err != nil {
			// Log error but continue processing
			fmt.Printf("Warning: failed to parse record: %v\n", err)
			continue
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// parseRecord converts a CSV record to SlackMessage
func (p *CSVParser) parseRecord(record []string, columnMap map[string]int) (SlackMessage, error) {
	msg := SlackMessage{}

	// Parse each field with error handling
	if idx, ok := columnMap["message_id"]; ok && idx < len(record) {
		msg.MessageID = record[idx]
	}

	if idx, ok := columnMap["timestamp"]; ok && idx < len(record) {
		// Parse timestamp - adjust format as needed
		if t, err := time.Parse(time.RFC3339, record[idx]); err == nil {
			msg.Timestamp = t
		}
	}

	if idx, ok := columnMap["channel"]; ok && idx < len(record) {
		msg.Channel = record[idx]
	}

	if idx, ok := columnMap["user"]; ok && idx < len(record) {
		msg.User = record[idx]
	}

	if idx, ok := columnMap["content"]; ok && idx < len(record) {
		msg.Content = record[idx]
	}

	if idx, ok := columnMap["thread_id"]; ok && idx < len(record) {
		msg.ThreadID = record[idx]
	}

	if idx, ok := columnMap["reactions"]; ok && idx < len(record) {
		msg.Reactions = record[idx]
	}

	return msg, nil
}
