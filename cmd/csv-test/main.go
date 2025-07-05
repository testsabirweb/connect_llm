package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/testsabirweb/connect_llm/pkg/ingestion"
	"github.com/testsabirweb/connect_llm/pkg/models"
)

func main() {
	var (
		csvFile      = flag.String("file", "slack/messages.csv", "Path to CSV file")
		batchSize    = flag.Int("batch", 100, "Batch size for processing")
		limit        = flag.Int("limit", 10, "Number of messages to display")
		skipErrors   = flag.Bool("skip-errors", true, "Skip records with errors")
		validate     = flag.Bool("validate", true, "Validate records")
		showMessages = flag.Bool("show-messages", false, "Show all messages in batches")
	)
	flag.Parse()

	// Create parser with configuration
	config := ingestion.ParserConfig{
		BatchSize:       *batchSize,
		SkipErrors:      *skipErrors,
		ValidateRecords: *validate,
	}
	parser := ingestion.NewCSVParser(config)

	// Track messages for display
	var displayMessages []models.SlackMessage
	totalMessages := 0

	// Define a callback to handle each batch of messages
	batchCallback := func(messages []models.SlackMessage, batchNum int) error {
		fmt.Printf("\n=== Batch %d ===\n", batchNum)
		fmt.Printf("Messages in batch: %d\n", len(messages))

		for i, msg := range messages {
			if *showMessages || i < 3 { // Show first 3 messages of each batch
				printMessage(msg)
			}
		}

		if !*showMessages && len(messages) > 3 {
			fmt.Printf("... and %d more messages\n", len(messages)-3)
		}
		return nil
	}

	// Parse file with callbacks
	err := parser.ParseFile(
		*csvFile,
		batchCallback,
		func(processed, total, errors int) {
			fmt.Printf("\rProgress: %d/%d messages processed, %d errors", processed, total, errors)
		},
	)

	fmt.Println() // New line after progress

	if err != nil {
		log.Fatalf("Error parsing file: %v", err)
	}

	// Print statistics
	total, processed, errorCount := parser.GetStats()
	fmt.Printf("\n=== Parsing Complete ===\n")
	fmt.Printf("Total records: %d\n", total)
	fmt.Printf("Processed successfully: %d\n", processed)
	fmt.Printf("Errors: %d\n", errorCount)

	// Print sample messages
	fmt.Printf("\n=== Sample Messages (first %d) ===\n", *limit)
	for i, msg := range displayMessages {
		fmt.Printf("\n--- Message %d ---\n", i+1)
		fmt.Printf("ID: %s\n", msg.MessageID)
		fmt.Printf("Timestamp: %s\n", msg.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("Channel: %s\n", msg.Channel)
		fmt.Printf("User: %s\n", msg.User)
		fmt.Printf("Type: %s\n", msg.Type)
		if msg.Subtype != "" {
			fmt.Printf("Subtype: %s\n", msg.Subtype)
		}
		if msg.ThreadTS != "" {
			fmt.Printf("Thread: %s\n", msg.ThreadTS)
		}
		if msg.ReplyCount > 0 {
			fmt.Printf("Replies: %d\n", msg.ReplyCount)
		}

		// Truncate content for display
		content := msg.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Printf("Content: %s\n", content)
	}

	// Print errors if any
	if errorCount > 0 && *skipErrors {
		errors := parser.GetErrors()
		fmt.Printf("\n=== First 10 Errors ===\n")
		for i, err := range errors {
			if i >= 10 {
				break
			}
			fmt.Printf("%d. %v\n", i+1, err)
		}
	}

	// Summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("CSV file: %s\n", *csvFile)

	fileInfo, err := os.Stat(*csvFile)
	if err == nil {
		fmt.Printf("File size: %.2f MB\n", float64(fileInfo.Size())/(1024*1024))
	}

	fmt.Printf("Total messages parsed: %d\n", totalMessages)
	if errorCount > 0 {
		fmt.Printf("Success rate: %.2f%%\n", float64(processed)/float64(total)*100)
	}
}

func printMessage(msg models.SlackMessage) {
	fmt.Printf("\n--- Message ---\n")
	fmt.Printf("ID: %s\n", msg.MessageID)
	fmt.Printf("Time: %s\n", msg.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Channel: %s\n", msg.Channel)
	fmt.Printf("User: %s\n", msg.User)
	fmt.Printf("Type: %s\n", msg.Type)
	if msg.Subtype != "" {
		fmt.Printf("Subtype: %s\n", msg.Subtype)
	}
	if msg.ThreadTS != "" {
		fmt.Printf("Thread: %s\n", msg.ThreadTS)
	}
	fmt.Printf("Content: %s\n", truncateString(msg.Content, 100))
	if msg.ReplyCount > 0 {
		fmt.Printf("Replies: %d\n", msg.ReplyCount)
	}
}

func truncateString(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength] + "..."
	}
	return s
}
