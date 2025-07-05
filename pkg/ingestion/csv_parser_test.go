package ingestion

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/testsabirweb/connect_llm/pkg/models"
)

func TestNewCSVParser(t *testing.T) {
	// Test with default config
	parser := NewCSVParser()
	if parser.config.BatchSize != 100 {
		t.Errorf("Expected default batch size 100, got %d", parser.config.BatchSize)
	}
	if !parser.config.SkipErrors {
		t.Error("Expected default SkipErrors to be true")
	}
	if !parser.config.ValidateRecords {
		t.Error("Expected default ValidateRecords to be true")
	}

	// Test with custom config
	config := ParserConfig{
		BatchSize:       50,
		SkipErrors:      false,
		ValidateRecords: false,
	}
	parser2 := NewCSVParser(config)
	if parser2.config.BatchSize != 50 {
		t.Errorf("Expected batch size 50, got %d", parser2.config.BatchSize)
	}
}

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		wantErr   bool
	}{
		{
			name:      "Valid timestamp with microseconds",
			timestamp: "1599934232.150700",
			wantErr:   false,
		},
		{
			name:      "Valid Unix timestamp without microseconds",
			timestamp: "1599934232",
			wantErr:   false,
		},
		{
			name:      "Valid datetime format",
			timestamp: "2020-09-12 18:10:32",
			wantErr:   false,
		},
		{
			name:      "Invalid format - not numeric",
			timestamp: "abc.def",
			wantErr:   true,
		},
		{
			name:      "Invalid format - empty string",
			timestamp: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := parseSlackTimestamp(tt.timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSlackTimestamp() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && ts.IsZero() {
				t.Error("parseSlackTimestamp() returned zero time for valid timestamp")
			}
		})
	}
}

func TestParseJSONArrayString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "Valid array",
			input: `["user1", "user2", "user3"]`,
			want:  []string{"user1", "user2", "user3"},
		},
		{
			name:  "Empty array",
			input: "[]",
			want:  nil,
		},
		{
			name:  "Null string",
			input: "null",
			want:  nil,
		},
		{
			name:  "Array with single quotes",
			input: `['user1', 'user2']`,
			want:  []string{"user1", "user2"},
		},
		{
			name:  "Array with spaces",
			input: `[ "user1" , "user2" ]`,
			want:  []string{"user1", "user2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJSONArrayString(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseJSONArrayString() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseJSONArrayString() = %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

func TestCSVParser_Parse(t *testing.T) {
	// Test CSV with valid data
	validCSV := `blocks,bot_id,channel_id,text,ts,type,user,thread_ts,subtype,reply_count,reply_users
null,,C01234567,Hello world,1599934232.150700,message,U01234567,,,0,[]
null,,C01234567,This is a reply,1599934240.150700,message,U87654321,1599934232.150700,,0,[]
null,,C01234567,<@U01234567> has joined the channel,1599934250.150700,message,U01234567,,channel_join,0,[]`

	parser := NewCSVParser()
	messages, err := parser.Parse(strings.NewReader(validCSV))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Check first message
	if messages[0].Content != "Hello world" {
		t.Errorf("Expected content 'Hello world', got '%s'", messages[0].Content)
	}
	if messages[0].User != "U01234567" {
		t.Errorf("Expected user 'U01234567', got '%s'", messages[0].User)
	}
	if messages[0].Channel != "C01234567" {
		t.Errorf("Expected channel 'C01234567', got '%s'", messages[0].Channel)
	}

	// Check threaded message
	if messages[1].ThreadTS != "1599934232.150700" {
		t.Errorf("Expected thread_ts '1599934232.150700', got '%s'", messages[1].ThreadTS)
	}

	// Check system message
	if messages[2].Subtype != "channel_join" {
		t.Errorf("Expected subtype 'channel_join', got '%s'", messages[2].Subtype)
	}
}

func TestCSVParser_ParseWithErrors(t *testing.T) {
	// Test CSV with some valid and some invalid records
	invalidCSV := `channel_id,text,ts,type,user
C01234567,Hello,1599934232.150700,message,U01234567
,No channel,1599934240.150700,message,U87654321
C01234567,Invalid timestamp,invalid_timestamp,message,U11111111
C01234567,No user,1599934250.150700,message,`

	parser := NewCSVParser(ParserConfig{
		SkipErrors:      true,
		ValidateRecords: true,
	})

	messages, err := parser.Parse(strings.NewReader(invalidCSV))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have processed some messages despite errors
	if len(messages) == 0 {
		t.Error("Expected some messages to be processed")
	}

	// The first valid message should have been processed
	if len(messages) >= 1 && messages[0].Content != "Hello" {
		t.Errorf("Expected first message content to be 'Hello', got '%s'", messages[0].Content)
	}

	// Check error count
	_, _, errorCount := parser.GetStats()
	if errorCount == 0 {
		t.Error("Expected some errors to be recorded")
	}

	errors := parser.GetErrors()
	if len(errors) == 0 {
		t.Error("Expected errors to be recorded")
	}
}

func TestCSVParser_BatchProcessing(t *testing.T) {
	// Create CSV with multiple records
	var csvBuilder strings.Builder
	csvBuilder.WriteString("blocks,bot_id,channel_id,text,ts,type,user,thread_ts,subtype,reply_count,reply_users\n")

	for i := 0; i < 250; i++ {
		csvBuilder.WriteString(fmt.Sprintf("null,,C01234567,Message %d,1599934232.%06d,message,U%08d,,,0,[]\n", i, i, i))
	}

	parser := NewCSVParser(ParserConfig{
		BatchSize: 100,
	})

	batchCount := 0
	totalMessages := 0

	err := parser.ParseWithCallbacks(
		strings.NewReader(csvBuilder.String()),
		0,
		func(messages []models.SlackMessage, batchNum int) error {
			batchCount++
			totalMessages += len(messages)

			// Verify batch size (except last batch)
			if batchNum < 2 && len(messages) != 100 {
				t.Errorf("Expected batch size 100, got %d for batch %d", len(messages), batchNum)
			}

			return nil
		},
		nil,
	)

	if err != nil {
		t.Fatalf("ParseWithCallbacks() error = %v", err)
	}

	// Should have 3 batches (100, 100, 50)
	if batchCount != 3 {
		t.Errorf("Expected 3 batches, got %d", batchCount)
	}

	if totalMessages != 250 {
		t.Errorf("Expected 250 total messages, got %d", totalMessages)
	}
}

func TestCSVParser_ProgressTracking(t *testing.T) {
	// Create CSV with multiple records
	var csvBuilder strings.Builder
	csvBuilder.WriteString("blocks,bot_id,channel_id,text,ts,type,user,thread_ts,subtype,reply_count,reply_users\n")

	for i := 0; i < 150; i++ {
		csvBuilder.WriteString(fmt.Sprintf("null,,C01234567,Message %d,1599934232.%06d,message,U%08d,,,0,[]\n", i, i, i))
	}

	parser := NewCSVParser()
	progressCalls := 0
	lastProcessed := 0

	err := parser.ParseWithCallbacks(
		strings.NewReader(csvBuilder.String()),
		0,
		func(messages []models.SlackMessage, batchNum int) error {
			return nil
		},
		func(processed, total, errors int) {
			progressCalls++
			if processed < lastProcessed {
				t.Error("Progress went backwards")
			}
			lastProcessed = processed
		},
	)

	if err != nil {
		t.Fatalf("ParseWithCallbacks() error = %v", err)
	}

	// Progress should be called at least once (final report)
	if progressCalls == 0 {
		t.Error("Progress callback was never called")
	}
}

func TestCSVParser_ValidateMessage(t *testing.T) {
	parser := NewCSVParser()

	tests := []struct {
		name    string
		message models.SlackMessage
		wantErr bool
	}{
		{
			name: "Valid message",
			message: models.SlackMessage{
				Content:   "Hello",
				User:      "U123",
				Channel:   "C123",
				Timestamp: time.Now(),
				Type:      "message",
			},
			wantErr: false,
		},
		{
			name: "Valid bot message",
			message: models.SlackMessage{
				Content:   "Bot says hello",
				BotID:     "B123",
				Channel:   "C123",
				Timestamp: time.Now(),
				Type:      "message",
			},
			wantErr: false,
		},
		{
			name: "Valid system message",
			message: models.SlackMessage{
				Content:   "<@U123> has joined",
				User:      "U123",
				Channel:   "C123",
				Timestamp: time.Now(),
				Type:      "message",
				Subtype:   "channel_join",
			},
			wantErr: false,
		},
		{
			name: "Missing user and bot",
			message: models.SlackMessage{
				Content:   "Hello",
				Channel:   "C123",
				Timestamp: time.Now(),
				Type:      "message",
			},
			wantErr: true,
		},
		{
			name: "Missing channel",
			message: models.SlackMessage{
				Content:   "Hello",
				User:      "U123",
				Timestamp: time.Now(),
				Type:      "message",
			},
			wantErr: true,
		},
		{
			name: "Missing timestamp",
			message: models.SlackMessage{
				Content: "Hello",
				User:    "U123",
				Channel: "C123",
				Type:    "message",
			},
			wantErr: true,
		},
		{
			name: "Empty content for regular message (now allowed)",
			message: models.SlackMessage{
				Content:   "",
				User:      "U123",
				Channel:   "C123",
				Timestamp: time.Now(),
				Type:      "message",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.validateMessage(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCSVParser_MissingRequiredColumns(t *testing.T) {
	// CSV missing required columns
	invalidCSV := `channel,message,timestamp
C123,Hello,2021-01-01`

	parser := NewCSVParser()
	_, err := parser.Parse(strings.NewReader(invalidCSV))

	if err == nil {
		t.Error("Expected error for missing required columns")
	}

	if !strings.Contains(err.Error(), "required column") {
		t.Errorf("Expected error about required column, got: %v", err)
	}
}
