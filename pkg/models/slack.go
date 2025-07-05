package models

import "time"

// SlackMessage represents a message from Slack export
type SlackMessage struct {
	MessageID string    `json:"message_id"`
	Timestamp time.Time `json:"timestamp"`
	Channel   string    `json:"channel"`
	User      string    `json:"user"`
	Content   string    `json:"content"`
	ThreadTS  string    `json:"thread_ts"`
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype"`
	// Additional fields for richer data
	ReplyCount   int      `json:"reply_count,omitempty"`
	ReplyUsers   []string `json:"reply_users,omitempty"`
	Reactions    string   `json:"reactions,omitempty"`
	ParentUserID string   `json:"parent_user_id,omitempty"`
	BotID        string   `json:"bot_id,omitempty"`
	FileIDs      []string `json:"file_ids,omitempty"`
}
