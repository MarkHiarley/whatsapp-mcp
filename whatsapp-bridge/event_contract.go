package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Event is the durable projection of one WhatsApp message, used by rules,
// cursors and webhooks. It is persisted in the tenant's messages.db.
type Event struct {
	Seq         int64     `json:"seq"`
	MessageID   string    `json:"message_id"`
	ChatJID     string    `json:"chat_jid"`
	ChatName    string    `json:"chat_name,omitempty"`
	SenderJID   string    `json:"sender_jid,omitempty"`
	SenderName  string    `json:"sender_name,omitempty"`
	IsGroup     bool      `json:"is_group"`
	IsFromMe    bool      `json:"is_from_me"`
	MentionedMe bool      `json:"mentioned_me"`
	Content     string    `json:"content"`
	MediaType   string    `json:"media_type,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
}

// MessageEventMeta carries the event-only fields extracted from a message.
// A nil meta means "do not publish an event" (history sync).
type MessageEventMeta struct {
	ChatName    string
	SenderJID   string
	SenderName  string
	IsGroup     bool
	MentionedMe bool
}

// StoredMessage is the frozen shape of a row written to messages.db.
type StoredMessage struct {
	ID        string
	ChatJID   string
	Sender    string
	Content   string
	Timestamp time.Time
	IsFromMe  bool

	MediaType     string
	Filename      string
	URL           string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    uint64
}

// RuleMatch is the AND of its non-empty fields; values inside a list are OR.
type RuleMatch struct {
	ChatJIDs     []string `json:"chat_jids,omitempty"`
	SenderJIDs   []string `json:"sender_jids,omitempty"`
	IsGroup      *bool    `json:"is_group,omitempty"`
	IsFromMe     *bool    `json:"is_from_me,omitempty"`
	MentionedMe  *bool    `json:"mentioned_me,omitempty"`
	TextContains []string `json:"text_contains,omitempty"`
	TextRegex    string   `json:"text_regex,omitempty"`
}

// EventRule is one saved filter.
type EventRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Match     RuleMatch `json:"match"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate rejects a malformed rule before it is persisted.
func (m RuleMatch) Validate() error {
	if _, err := regexp.Compile(m.TextRegex); err != nil && m.TextRegex != "" {
		return fmt.Errorf("invalid text_regex: %w", err)
	}
	for _, jid := range m.ChatJIDs {
		if strings.TrimSpace(jid) == "" {
			return fmt.Errorf("chat_jids cannot contain an empty value")
		}
	}
	for _, jid := range m.SenderJIDs {
		if strings.TrimSpace(jid) == "" {
			return fmt.Errorf("sender_jids cannot contain an empty value")
		}
	}
	for _, text := range m.TextContains {
		if text == "" {
			return fmt.Errorf("text_contains cannot contain an empty value")
		}
	}
	return nil
}

// Matches evaluates the rule against one event.
func (m RuleMatch) Matches(ev Event) bool {
	if len(m.ChatJIDs) > 0 && !matchesAnyJID(ev.ChatJID, m.ChatJIDs) {
		return false
	}
	if len(m.SenderJIDs) > 0 && !matchesAnyJID(ev.SenderJID, m.SenderJIDs) {
		return false
	}
	if m.IsGroup != nil && ev.IsGroup != *m.IsGroup {
		return false
	}
	if m.IsFromMe != nil && ev.IsFromMe != *m.IsFromMe {
		return false
	}
	if m.MentionedMe != nil && ev.MentionedMe != *m.MentionedMe {
		return false
	}
	if len(m.TextContains) > 0 {
		content := strings.ToLower(ev.Content)
		matched := false
		for _, want := range m.TextContains {
			if strings.Contains(content, strings.ToLower(want)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if m.TextRegex != "" {
		re, err := regexp.Compile(m.TextRegex)
		if err != nil || !re.MatchString(ev.Content) {
			return false
		}
	}
	return true
}

func matchesAnyJID(actual string, wants []string) bool {
	for _, want := range wants {
		if jidMatches(actual, want) {
			return true
		}
	}
	return false
}

// jidMatches accepts a full JID or the bare user part. Group JIDs must match
// the whole string, because group user IDs are not phone numbers.
func jidMatches(actual, want string) bool {
	if actual == "" || want == "" {
		return false
	}
	if actual == want {
		return true
	}
	if strings.HasSuffix(actual, "@g.us") || strings.HasSuffix(want, "@g.us") {
		return false
	}
	return jidUser(actual) == jidUser(want)
}

func jidUser(jid string) string {
	if i := strings.IndexByte(jid, '@'); i >= 0 {
		return jid[:i]
	}
	return jid
}

// migrateEventSchema freezes the event tables in every tenant messages.db.
func migrateEventSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS events (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			chat_name TEXT,
			sender_jid TEXT,
			sender_name TEXT,
			is_group INTEGER NOT NULL DEFAULT 0,
			is_from_me INTEGER NOT NULL DEFAULT 0,
			mentioned_me INTEGER NOT NULL DEFAULT 0,
			content TEXT,
			media_type TEXT,
			timestamp DATETIME,
			created_at DATETIME,
			UNIQUE(message_id, chat_jid)
		)`,
		`CREATE TABLE IF NOT EXISTS event_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			match_json TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS event_consumers (
			consumer_id TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			last_seq INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (consumer_id, rule_id)
		)`,
		`CREATE TABLE IF NOT EXISTS event_webhooks (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			secret TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS event_deliveries (
			seq INTEGER NOT NULL,
			webhook_id TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			last_status INTEGER,
			last_error TEXT,
			next_attempt_at DATETIME,
			delivered_at DATETIME,
			PRIMARY KEY (seq, webhook_id)
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to migrate event schema: %w", err)
		}
	}
	return nil
}
