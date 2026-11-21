package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// eventIDPattern is shared by rule ids, consumer ids and webhook ids.
var eventIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidEventID reports whether id is an acceptable rule/consumer/webhook id.
func ValidEventID(id string) bool {
	return eventIDPattern.MatchString(id)
}

// newEventID creates a prefixed identifier such as rule_0123456789abcdef.
func newEventID(prefix string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}

// listRules returns every rule ordered by creation time.
func (store *MessageStore) listRules() ([]EventRule, error) {
	rows, err := store.db.Query(`
		SELECT id, name, match_json, enabled, created_at
		FROM event_rules
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]EventRule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// getRule loads one rule; the boolean reports existence.
func (store *MessageStore) getRule(id string) (EventRule, bool, error) {
	row := store.db.QueryRow(`
		SELECT id, name, match_json, enabled, created_at
		FROM event_rules WHERE id = ?
	`, id)
	rule, err := scanRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return EventRule{}, false, nil
	}
	if err != nil {
		return EventRule{}, false, err
	}
	return rule, true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRule(row rowScanner) (EventRule, error) {
	var (
		rule      EventRule
		matchJSON string
	)
	if err := row.Scan(&rule.ID, &rule.Name, &matchJSON, &rule.Enabled, &rule.CreatedAt); err != nil {
		return EventRule{}, err
	}
	if err := json.Unmarshal([]byte(matchJSON), &rule.Match); err != nil {
		return EventRule{}, fmt.Errorf("invalid match_json for rule %s: %w", rule.ID, err)
	}
	return rule, nil
}

// insertRule persists a validated rule.
func (store *MessageStore) insertRule(rule EventRule) error {
	matchJSON, err := json.Marshal(rule.Match)
	if err != nil {
		return err
	}
	_, err = store.db.Exec(`
		INSERT INTO event_rules (id, name, match_json, enabled, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, rule.ID, rule.Name, string(matchJSON), rule.Enabled, rule.CreatedAt)
	return err
}

// updateRule rewrites the mutable fields; the boolean reports existence.
func (store *MessageStore) updateRule(rule EventRule) (bool, error) {
	matchJSON, err := json.Marshal(rule.Match)
	if err != nil {
		return false, err
	}
	result, err := store.db.Exec(`
		UPDATE event_rules SET name = ?, match_json = ?, enabled = ? WHERE id = ?
	`, rule.Name, string(matchJSON), rule.Enabled, rule.ID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// deleteRule removes one rule; the boolean reports existence.
func (store *MessageStore) deleteRule(id string) (bool, error) {
	result, err := store.db.Exec("DELETE FROM event_rules WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// consumerCursor returns the persisted last_seq, or 0 when no cursor exists.
func (store *MessageStore) consumerCursor(consumerID, ruleID string) (int64, error) {
	var seq int64
	err := store.db.QueryRow(`
		SELECT last_seq FROM event_consumers WHERE consumer_id = ? AND rule_id = ?
	`, consumerID, ruleID).Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return seq, nil
}

// ackConsumer persists a monotonic cursor and returns the stored value.
func (store *MessageStore) ackConsumer(consumerID, ruleID string, seq int64) (int64, error) {
	now := time.Now().UTC()
	_, err := store.db.Exec(`
		INSERT INTO event_consumers (consumer_id, rule_id, last_seq, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(consumer_id, rule_id) DO UPDATE SET
			last_seq = MAX(last_seq, excluded.last_seq),
			updated_at = excluded.updated_at
	`, consumerID, ruleID, seq, now)
	if err != nil {
		return 0, err
	}
	return store.consumerCursor(consumerID, ruleID)
}

const eventColumns = `seq, message_id, chat_jid, chat_name, sender_jid, sender_name,
	is_group, is_from_me, mentioned_me, content, media_type, timestamp, created_at`

func scanEvent(row rowScanner) (Event, error) {
	var (
		ev                              Event
		chatName, senderJID, senderName sql.NullString
		content, mediaType              sql.NullString
		timestamp, createdAt            sql.NullTime
	)
	if err := row.Scan(
		&ev.Seq, &ev.MessageID, &ev.ChatJID, &chatName, &senderJID, &senderName,
		&ev.IsGroup, &ev.IsFromMe, &ev.MentionedMe, &content, &mediaType, &timestamp, &createdAt,
	); err != nil {
		return Event{}, err
	}
	ev.ChatName = chatName.String
	ev.SenderJID = senderJID.String
	ev.SenderName = senderName.String
	ev.Content = content.String
	ev.MediaType = mediaType.String
	if timestamp.Valid {
		ev.Timestamp = timestamp.Time
	}
	if createdAt.Valid {
		ev.CreatedAt = createdAt.Time
	}
	return ev, nil
}

// listEventsAfter returns up to limit events with seq > after, ordered by seq.
func (store *MessageStore) listEventsAfter(after int64, limit int) ([]Event, error) {
	rows, err := store.db.Query(`
		SELECT `+eventColumns+`
		FROM events WHERE seq > ?
		ORDER BY seq ASC LIMIT ?
	`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0, limit)
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

// nextEventAfter returns the first event with seq > after.
func (store *MessageStore) nextEventAfter(after int64) (Event, bool, error) {
	row := store.db.QueryRow(`
		SELECT `+eventColumns+`
		FROM events WHERE seq > ?
		ORDER BY seq ASC LIMIT 1
	`, after)
	ev, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, err
	}
	return ev, true, nil
}
