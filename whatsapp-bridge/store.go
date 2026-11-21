package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// Message represents a chat message for our client
type Message struct {
	Time      time.Time
	Sender    string
	Content   string
	IsFromMe  bool
	MediaType string
	Filename  string
}

// Database handler for storing message history
type MessageStore struct {
	db       *sql.DB
	mediaDir string
}

// NewMessageStoreAt opens the message database and media directory of one tenant.
func NewMessageStoreAt(dbPath, mediaDir string) (*MessageStore, error) {
	if dbPath == "" || mediaDir == "" {
		return nil, fmt.Errorf("message database path and media directory are required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create message database directory: %v", err)
	}
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create media directory: %v", err)
	}

	// Open SQLite database for messages
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open message database: %v", err)
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT,
			last_message_time TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT,
			chat_jid TEXT,
			sender TEXT,
			content TEXT,
			timestamp TIMESTAMP,
			is_from_me BOOLEAN,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		);

		CREATE TABLE IF NOT EXISTS contacts (
			phone TEXT PRIMARY KEY,
			lid TEXT,
			display_name TEXT,
			push_name TEXT
		);
		CREATE INDEX IF NOT EXISTS contacts_lid_idx ON contacts(lid);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	if err := migrateEventSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &MessageStore{db: db, mediaDir: mediaDir}, nil
}

// NewMessageStore opens the legacy default store.
func NewMessageStore() (*MessageStore, error) {
	return NewMessageStoreAt("store/messages.db", "store")
}

// MediaDir is the directory that receives this tenant's downloaded media.
func (store *MessageStore) MediaDir() string {
	return store.mediaDir
}

// Close the database connection
func (store *MessageStore) Close() error {
	return store.db.Close()
}

// SyncContacts copies WhatsApp's contact names and LID mappings into messages.db.
func (store *MessageStore) SyncContacts(client *whatsmeow.Client) error {
	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return err
	}

	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for jid, contact := range contacts {
		var phone, lid string
		switch jid.Server {
		case types.DefaultUserServer:
			phone = jid.User
			if mapped, mapErr := client.Store.LIDs.GetLIDForPN(context.Background(), jid.ToNonAD()); mapErr == nil {
				lid = mapped.User
			}
		case types.HiddenUserServer:
			lid = jid.User
			if mapped, mapErr := client.Store.LIDs.GetPNForLID(context.Background(), jid.ToNonAD()); mapErr == nil {
				phone = mapped.User
			}
		}
		if phone == "" {
			continue
		}

		displayName := contact.FullName
		if displayName == "" {
			displayName = contact.BusinessName
		}
		if displayName == "" {
			displayName = contact.FirstName
		}
		_, err = tx.Exec(`
			INSERT INTO contacts (phone, lid, display_name, push_name) VALUES (?, ?, ?, ?)
			ON CONFLICT(phone) DO UPDATE SET
				lid = COALESCE(NULLIF(excluded.lid, ''), contacts.lid),
				display_name = COALESCE(NULLIF(excluded.display_name, ''), contacts.display_name),
				push_name = COALESCE(NULLIF(excluded.push_name, ''), contacts.push_name)
		`, phone, lid, displayName, contact.PushName)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CanonicalJID converts a privacy LID into its phone-number JID when known.
func (store *MessageStore) CanonicalJID(client *whatsmeow.Client, jid types.JID) types.JID {
	jid = jid.ToNonAD()
	if jid.Server != types.HiddenUserServer {
		return jid
	}
	phoneJID, err := client.Store.LIDs.GetPNForLID(context.Background(), jid)
	if err != nil || phoneJID.IsEmpty() {
		return jid
	}
	_, _ = store.db.Exec(`
		INSERT INTO contacts (phone, lid) VALUES (?, ?)
		ON CONFLICT(phone) DO UPDATE SET lid = excluded.lid
	`, phoneJID.User, jid.User)
	return phoneJID.ToNonAD()
}

func (store *MessageStore) ContactName(jid types.JID) string {
	var name string
	_ = store.db.QueryRow(`
		SELECT COALESCE(NULLIF(display_name, ''), NULLIF(push_name, ''), phone)
		FROM contacts WHERE phone = ? OR lid = ? LIMIT 1
	`, jid.User, jid.User).Scan(&name)
	return name
}

// Store a chat in the database
func (store *MessageStore) StoreChat(jid, name string, lastMessageTime time.Time) error {
	_, err := store.db.Exec(
		"INSERT OR REPLACE INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)",
		jid, name, lastMessageTime,
	)
	return err
}

// StoreMessage writes the message and (when meta is non-nil) its event in one
// transaction. It returns the event sequence, or 0 when no event was written.
func (store *MessageStore) StoreMessage(m StoredMessage, meta *MessageEventMeta) (int64, error) {
	// Only store if there's actual content or media
	if m.Content == "" && m.MediaType == "" {
		return 0, nil
	}

	tx, err := store.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT OR REPLACE INTO messages 
		(id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ChatJID, m.Sender, m.Content, m.Timestamp, m.IsFromMe, m.MediaType, m.Filename, m.URL,
		m.MediaKey, m.FileSHA256, m.FileEncSHA256, m.FileLength,
	)
	if err != nil {
		return 0, err
	}

	var seq int64
	if meta != nil {
		_, err = tx.Exec(
			`INSERT OR IGNORE INTO events
			(message_id, chat_jid, chat_name, sender_jid, sender_name, is_group, chat_type, is_from_me, mentioned_me, is_reply_to_me, message_type, content, media_type, timestamp, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.ChatJID, meta.ChatName, meta.SenderJID, meta.SenderName, meta.IsGroup, meta.ChatType, m.IsFromMe,
			meta.MentionedMe, meta.IsReplyToMe, meta.MessageType, m.Content, m.MediaType, m.Timestamp, time.Now().UTC(),
		)
		if err != nil {
			return 0, err
		}
		if err := tx.QueryRow(
			"SELECT seq FROM events WHERE message_id = ? AND chat_jid = ?",
			m.ID, m.ChatJID,
		).Scan(&seq); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return seq, nil
}

// Get messages from a chat
func (store *MessageStore) GetMessages(chatJID string, limit int) ([]Message, error) {
	rows, err := store.db.Query(
		"SELECT sender, content, timestamp, is_from_me, media_type, filename FROM messages WHERE chat_jid = ? ORDER BY timestamp DESC LIMIT ?",
		chatJID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp time.Time
		err := rows.Scan(&msg.Sender, &msg.Content, &timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename)
		if err != nil {
			return nil, err
		}
		msg.Time = timestamp
		messages = append(messages, msg)
	}

	return messages, nil
}

// Get all chats
func (store *MessageStore) GetChats() (map[string]time.Time, error) {
	rows, err := store.db.Query("SELECT jid, last_message_time FROM chats ORDER BY last_message_time DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make(map[string]time.Time)
	for rows.Next() {
		var jid string
		var lastMessageTime time.Time
		err := rows.Scan(&jid, &lastMessageTime)
		if err != nil {
			return nil, err
		}
		chats[jid] = lastMessageTime
	}

	return chats, nil
}

// Store additional media info in the database
func (store *MessageStore) StoreMediaInfo(id, chatJID, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	_, err := store.db.Exec(
		"UPDATE messages SET url = ?, media_key = ?, file_sha256 = ?, file_enc_sha256 = ?, file_length = ? WHERE id = ? AND chat_jid = ?",
		url, mediaKey, fileSHA256, fileEncSHA256, fileLength, id, chatJID,
	)
	return err
}

// Get media info from the database
func (store *MessageStore) GetMediaInfo(id, chatJID string) (string, string, string, []byte, []byte, []byte, uint64, error) {
	var mediaType, filename, url string
	var mediaKey, fileSHA256, fileEncSHA256 []byte
	var fileLength uint64

	err := store.db.QueryRow(
		"SELECT media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length FROM messages WHERE id = ? AND chat_jid = ?",
		id, chatJID,
	).Scan(&mediaType, &filename, &url, &mediaKey, &fileSHA256, &fileEncSHA256, &fileLength)

	return mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength, err
}
