package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func boolPtr(v bool) *bool { return &v }

func baseEvent() Event {
	return Event{
		Seq:       1,
		MessageID: "msg-1",
		ChatJID:   "120363000000000000@g.us",
		SenderJID: "5511999999999@s.whatsapp.net",
		IsGroup:   true,
		Content:   "Novo pedido #42",
	}
}

func TestRuleMatchChatJID(t *testing.T) {
	ev := baseEvent()
	if !(RuleMatch{ChatJIDs: []string{"120363000000000000@g.us"}}).Matches(ev) {
		t.Fatal("exact group JID should match")
	}
	if (RuleMatch{ChatJIDs: []string{"120363000000000001@g.us"}}).Matches(ev) {
		t.Fatal("another group JID must not match")
	}
	if (RuleMatch{ChatJIDs: []string{"120363000000000000"}}).Matches(ev) {
		t.Fatal("bare group user must not match a @g.us chat")
	}
}

func TestRuleMatchSenderBarePhone(t *testing.T) {
	ev := baseEvent()
	if !(RuleMatch{SenderJIDs: []string{"5511999999999"}}).Matches(ev) {
		t.Fatal("bare phone should match the sender user part")
	}
	if !(RuleMatch{SenderJIDs: []string{"5511999999999@lid"}}).Matches(ev) {
		t.Fatal("same user on another server should match")
	}
	if (RuleMatch{SenderJIDs: []string{"5511888888888"}}).Matches(ev) {
		t.Fatal("another sender must not match")
	}
}

func TestRuleMatchTextAndFlags(t *testing.T) {
	ev := baseEvent()
	if !(RuleMatch{TextContains: []string{"PEDIDO"}}).Matches(ev) {
		t.Fatal("text_contains must be case-insensitive")
	}
	if (RuleMatch{TextContains: []string{"reembolso"}}).Matches(ev) {
		t.Fatal("absent text must not match")
	}
	if !(RuleMatch{TextRegex: `#\d+$`}).Matches(ev) {
		t.Fatal("valid regex should match")
	}
	if (RuleMatch{TextRegex: `^reembolso`}).Matches(ev) {
		t.Fatal("non-matching regex must not match")
	}
	if !(RuleMatch{IsGroup: boolPtr(true)}).Matches(ev) {
		t.Fatal("is_group=true should match a group event")
	}
	if (RuleMatch{IsFromMe: boolPtr(true)}).Matches(ev) {
		t.Fatal("is_from_me=true must not match an inbound event")
	}
	ev.MentionedMe = true
	if !(RuleMatch{MentionedMe: boolPtr(true)}).Matches(ev) {
		t.Fatal("mentioned_me should match")
	}
}

func TestRuleMatchMediaAndReply(t *testing.T) {
	ev := baseEvent()
	ev.MediaType = "image"
	ev.IsReplyToMe = true

	if !(RuleMatch{HasMedia: boolPtr(true)}).Matches(ev) {
		t.Fatal("has_media=true should match an image event")
	}
	if (RuleMatch{HasMedia: boolPtr(false)}).Matches(ev) {
		t.Fatal("has_media=false must not match an image event")
	}
	if !(RuleMatch{MediaTypes: []string{"image"}}).Matches(ev) {
		t.Fatal("media_types=image should match")
	}
	if (RuleMatch{MediaTypes: []string{"audio"}}).Matches(ev) {
		t.Fatal("media_types=audio must not match an image event")
	}
	if !(RuleMatch{IsReplyToMe: boolPtr(true)}).Matches(ev) {
		t.Fatal("is_reply_to_me=true should match")
	}
	if (RuleMatch{IsReplyToMe: boolPtr(false)}).Matches(ev) {
		t.Fatal("is_reply_to_me=false must not match a reply")
	}
}

func TestRuleMatchEmptyMatchesEverything(t *testing.T) {
	if !(RuleMatch{}).Matches(baseEvent()) {
		t.Fatal("an empty rule is an explicit catch-all")
	}
}

func TestRuleValidate(t *testing.T) {
	if err := (RuleMatch{TextRegex: `(`}).Validate(); err == nil {
		t.Fatal("invalid regex must be rejected")
	}
	if err := (RuleMatch{ChatJIDs: []string{" "}}).Validate(); err == nil {
		t.Fatal("blank chat JID must be rejected")
	}
	if err := (RuleMatch{TextContains: []string{""}}).Validate(); err == nil {
		t.Fatal("blank text_contains must be rejected")
	}
	if err := (RuleMatch{TextRegex: `^ok$`}).Validate(); err != nil {
		t.Fatalf("valid rule rejected: %v", err)
	}
}

func TestEventSchemaMigration(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := migrateEventSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := migrateEventSchema(db); err != nil {
		t.Fatalf("migration must be idempotent: %v", err)
	}
	for _, table := range []string{"events", "event_rules", "event_consumers", "event_webhooks", "event_deliveries"} {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestEventUniquePerMessage(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrateEventSchema(db); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO events (message_id, chat_jid, content) VALUES (?, ?, ?)`
	if _, err := db.Exec(insert, "m1", "5511@s.whatsapp.net", "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(insert, "m1", "5511@s.whatsapp.net", "b"); err == nil {
		t.Fatal("duplicate (message_id, chat_jid) must be rejected")
	}
}

func TestRuleMatchChatAndMessageTypes(t *testing.T) {
	ev := baseEvent()
	ev.ChatType = "group"
	ev.MessageType = "image"
	if !(RuleMatch{ChatTypes: []string{"group"}}).Matches(ev) {
		t.Fatal("chat_types=group should match")
	}
	if (RuleMatch{ChatTypes: []string{"direct"}}).Matches(ev) {
		t.Fatal("chat_types=direct must not match a group")
	}
	if !(RuleMatch{MessageTypes: []string{"image"}}).Matches(ev) {
		t.Fatal("message_types=image should match")
	}
	if (RuleMatch{MessageTypes: []string{"text"}}).Matches(ev) {
		t.Fatal("message_types=text must not match an image")
	}
	if err := (RuleMatch{ChatTypes: []string{"channel"}}).Validate(); err == nil {
		t.Fatal("unknown chat type must be rejected")
	}
	if err := (RuleMatch{MessageTypes: []string{"unknown"}}).Validate(); err == nil {
		t.Fatal("unknown message type must be rejected")
	}
}

func TestWebhookAcceptsSafeDefaults(t *testing.T) {
	direct := baseEvent()
	direct.ChatJID = "5511999999999@s.whatsapp.net"
	direct.IsGroup = false
	direct.ChatType = "direct"
	direct.MessageType = "text"

	group := baseEvent()
	group.ChatType = "group"
	group.MessageType = "text"

	status := baseEvent()
	status.ChatJID = "status@broadcast"
	status.IsGroup = false
	status.ChatType = "status"
	status.MessageType = "image"

	if !webhookAccepts(RuleMatch{}, direct) {
		t.Fatal("an empty rule must accept a direct inbound message")
	}
	if webhookAccepts(RuleMatch{}, group) {
		t.Fatal("an empty rule must not accept groups")
	}
	if webhookAccepts(RuleMatch{}, status) {
		t.Fatal("an empty rule must not accept status")
	}
	if !webhookAccepts(RuleMatch{ChatTypes: []string{"group"}}, group) {
		t.Fatal("explicit chat_types=group must opt in")
	}
	if !webhookAccepts(RuleMatch{ChatTypes: []string{"status"}}, status) {
		t.Fatal("explicit chat_types=status must opt in")
	}
	if !webhookAccepts(RuleMatch{ChatJIDs: []string{group.ChatJID}}, group) {
		t.Fatal("an explicit group chat_jid must opt in")
	}

	outgoing := direct
	outgoing.IsFromMe = true
	if webhookAccepts(RuleMatch{}, outgoing) {
		t.Fatal("outgoing messages must be skipped by default")
	}
	if !webhookAccepts(RuleMatch{IsFromMe: boolPtr(true)}, outgoing) {
		t.Fatal("is_from_me=true must opt into outgoing messages")
	}
}

func TestChatTypeFromJID(t *testing.T) {
	cases := map[string]string{
		"5511999999999@s.whatsapp.net":  "direct",
		"120363000000000000@g.us":       "group",
		"120363000000000000@newsletter": "newsletter",
		"status@broadcast":              "status",
		"120363000000000000@broadcast":  "broadcast",
	}
	for jid, want := range cases {
		if got := chatTypeFromJID(jid); got != want {
			t.Fatalf("chatTypeFromJID(%q) = %q, want %q", jid, got, want)
		}
	}
}

func TestEventSchemaBackfillsSegmentation(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE events (
		seq INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT NOT NULL,
		chat_jid TEXT NOT NULL,
		content TEXT,
		media_type TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO events (message_id, chat_jid, content, media_type) VALUES ('m1', '120363000000000000@g.us', 'oi', ''), ('m2', '5511999999999@s.whatsapp.net', '', 'image')`); err != nil {
		t.Fatal(err)
	}
	if err := migrateEventSchema(db); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("SELECT chat_type, message_type FROM events ORDER BY seq")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var chatType, messageType string
		if err := rows.Scan(&chatType, &messageType); err != nil {
			t.Fatal(err)
		}
		got = append(got, chatType+":"+messageType)
	}
	if len(got) != 2 || got[0] != "group:text" || got[1] != "direct:image" {
		t.Fatalf("unexpected backfill: %v", got)
	}
}
