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
	ChatType    string    `json:"chat_type"`
	IsFromMe    bool      `json:"is_from_me"`
	MentionedMe bool      `json:"mentioned_me"`
	IsReplyToMe bool      `json:"is_reply_to_me"`
	MessageType string    `json:"message_type"`
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
	ChatType    string
	MentionedMe bool
	IsReplyToMe bool
	MessageType string
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
	IsReplyToMe  *bool    `json:"is_reply_to_me,omitempty"`
	ChatTypes    []string `json:"chat_types,omitempty"`
	MessageTypes []string `json:"message_types,omitempty"`
	HasMedia     *bool    `json:"has_media,omitempty"`
	MediaTypes   []string `json:"media_types,omitempty"`
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
	for _, chatType := range m.ChatTypes {
		if !validChatTypes[chatType] {
			return fmt.Errorf("invalid chat_type %q", chatType)
		}
	}
	for _, messageType := range m.MessageTypes {
		if !validMessageTypes[messageType] {
			return fmt.Errorf("invalid message_type %q", messageType)
		}
	}
	return nil
}

var validChatTypes = map[string]bool{
	"direct": true, "group": true, "status": true, "broadcast": true, "newsletter": true, "other": true,
}

var validMessageTypes = map[string]bool{
	"text": true, "image": true, "video": true, "audio": true, "document": true,
	"sticker": true, "location": true, "contact": true, "poll": true, "poll_update": true,
	"reaction": true, "protocol": true, "interactive": true, "album": true, "event": true, "other": true,
}

func chatTypeFromJID(jid string) string {
	switch {
	case strings.HasSuffix(jid, "@g.us"):
		return "group"
	case strings.HasSuffix(jid, "@newsletter"):
		return "newsletter"
	case jid == "status@broadcast":
		return "status"
	case strings.HasSuffix(jid, "@broadcast"):
		return "broadcast"
	case strings.HasSuffix(jid, "@s.whatsapp.net"), strings.HasSuffix(jid, "@lid"):
		return "direct"
	default:
		return "other"
	}
}

func messageTypeFromEvent(ev Event) string {
	if ev.MessageType != "" {
		return ev.MessageType
	}
	if ev.MediaType != "" {
		return ev.MediaType
	}
	if ev.Content != "" {
		return "text"
	}
	return "other"
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
	if m.IsReplyToMe != nil && ev.IsReplyToMe != *m.IsReplyToMe {
		return false
	}
	chatType := ev.ChatType
	if chatType == "" {
		chatType = chatTypeFromJID(ev.ChatJID)
	}
	if len(m.ChatTypes) > 0 && !matchesFold(chatType, m.ChatTypes) {
		return false
	}
	if len(m.MessageTypes) > 0 && !matchesFold(messageTypeFromEvent(ev), m.MessageTypes) {
		return false
	}
	if m.HasMedia != nil && (ev.MediaType != "") != *m.HasMedia {
		return false
	}
	if len(m.MediaTypes) > 0 {
		matched := false
		for _, want := range m.MediaTypes {
			if strings.EqualFold(ev.MediaType, want) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
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

func matchesFold(actual string, wants []string) bool {
	for _, want := range wants {
		if strings.EqualFold(actual, want) {
			return true
		}
	}
	return false
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
			chat_type TEXT NOT NULL DEFAULT '',
			is_from_me INTEGER NOT NULL DEFAULT 0,
			mentioned_me INTEGER NOT NULL DEFAULT 0,
			is_reply_to_me INTEGER NOT NULL DEFAULT 0,
			message_type TEXT NOT NULL DEFAULT '',
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
			headers_json TEXT NOT NULL DEFAULT '{}',
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
	for _, column := range []struct{ name, ddl string }{
		{"is_reply_to_me", "ALTER TABLE events ADD COLUMN is_reply_to_me INTEGER NOT NULL DEFAULT 0"},
		{"chat_type", "ALTER TABLE events ADD COLUMN chat_type TEXT NOT NULL DEFAULT ''"},
		{"message_type", "ALTER TABLE events ADD COLUMN message_type TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(db, "events", column.name, column.ddl); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`
		UPDATE events SET chat_type = CASE
			WHEN chat_jid LIKE '%@g.us' THEN 'group'
			WHEN chat_jid LIKE '%@newsletter' THEN 'newsletter'
			WHEN chat_jid = 'status@broadcast' THEN 'status'
			WHEN chat_jid LIKE '%@broadcast' THEN 'broadcast'
			WHEN chat_jid LIKE '%@lid' OR chat_jid LIKE '%@s.whatsapp.net' THEN 'direct'
			ELSE 'other' END
		WHERE chat_type = ''
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		UPDATE events SET message_type = CASE
			WHEN media_type IS NOT NULL AND media_type <> '' THEN media_type
			WHEN content IS NOT NULL AND content <> '' THEN 'text'
			ELSE 'other' END
		WHERE message_type = ''
	`); err != nil {
		return err
	}
	return ensureColumn(db, "event_webhooks", "headers_json", "ALTER TABLE event_webhooks ADD COLUMN headers_json TEXT NOT NULL DEFAULT '{}'")
}

func ensureColumn(db *sql.DB, table, column, ddl string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(ddl)
	return err
}
