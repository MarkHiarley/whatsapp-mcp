package main

import (
	"database/sql"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type chatListItem struct {
	JID             string     `json:"jid"`
	Name            string     `json:"name"`
	LastMessageTime *time.Time `json:"last_message_time,omitempty"`
	LastMessage     string     `json:"last_message,omitempty"`
	LastSender      string     `json:"last_sender,omitempty"`
	LastIsFromMe    bool       `json:"last_is_from_me"`
	LastMediaType   string     `json:"last_media_type,omitempty"`
	IsGroup         bool       `json:"is_group"`
}

type messageListItem struct {
	ID        string    `json:"id"`
	ChatJID   string    `json:"chat_jid"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	IsFromMe  bool      `json:"is_from_me"`
	MediaType string    `json:"media_type,omitempty"`
	Filename  string    `json:"filename,omitempty"`
}

type tenantStats struct {
	TenantID        string         `json:"tenant_id"`
	Status          TenantStatus   `json:"status"`
	Chats           int            `json:"chats"`
	Messages        int            `json:"messages"`
	Events          int            `json:"events"`
	Keys            int            `json:"keys"`
	LastMessageTime *time.Time     `json:"last_message_time,omitempty"`
	MessagesByDay   []dailyMessage `json:"messages_by_day"`
}

type dailyMessage struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// runtimeFor resolves the tenant path, checks ownership and returns the live runtime.
func (a *tenantAPI) runtimeFor(w http.ResponseWriter, p principal, tenantID string) (*tenantRuntime, bool) {
	tenantID, ok := a.authorize(w, p, tenantID)
	if !ok {
		return nil, false
	}
	rt, ok := a.m.Runtime(tenantID)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return nil, false
	}
	return rt, true
}

func (a *tenantAPI) handleTenantChats(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.runtimeFor(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	limit := clampInt(queryInt(r, "limit", 50), 1, 200)
	offset := queryInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	search := strings.TrimSpace(r.URL.Query().Get("query"))

	query := `SELECT c.jid, COALESCE(c.name, ''), c.last_message_time,
		(SELECT m.content FROM messages m WHERE m.chat_jid = c.jid ORDER BY m.timestamp DESC, m.id DESC LIMIT 1),
		(SELECT m.sender FROM messages m WHERE m.chat_jid = c.jid ORDER BY m.timestamp DESC, m.id DESC LIMIT 1),
		(SELECT m.is_from_me FROM messages m WHERE m.chat_jid = c.jid ORDER BY m.timestamp DESC, m.id DESC LIMIT 1),
		(SELECT m.media_type FROM messages m WHERE m.chat_jid = c.jid ORDER BY m.timestamp DESC, m.id DESC LIMIT 1)
		FROM chats c`
	args := []any{}
	if search != "" {
		query += ` WHERE c.name LIKE ? OR c.jid LIKE ?`
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	query += ` ORDER BY c.last_message_time DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := rt.store.db.Query(query, args...)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	defer rows.Close()

	chats := make([]chatListItem, 0, limit)
	for rows.Next() {
		var item chatListItem
		var lastTime sql.NullTime
		var lastMessage, lastSender, lastMedia sql.NullString
		var lastFromMe sql.NullBool
		if err := rows.Scan(&item.JID, &item.Name, &lastTime, &lastMessage, &lastSender, &lastFromMe, &lastMedia); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		if lastTime.Valid {
			t := lastTime.Time
			item.LastMessageTime = &t
		}
		item.LastMessage = lastMessage.String
		item.LastSender = lastSender.String
		item.LastIsFromMe = lastFromMe.Bool
		item.LastMediaType = lastMedia.String
		item.IsGroup = strings.HasSuffix(item.JID, "@g.us")
		chats = append(chats, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		Chats []chatListItem `json:"chats"`
	}{Chats: chats})
}

func (a *tenantAPI) handleTenantChatMessages(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.runtimeFor(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	chatJID := r.PathValue("jid")
	if chatJID == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Chat JID is required")
		return
	}
	limit := clampInt(queryInt(r, "limit", 50), 1, 200)

	where := "chat_jid = ?"
	args := []any{chatJID}
	if raw := strings.TrimSpace(r.URL.Query().Get("before")); raw != "" {
		before, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "before must be an ISO-8601 timestamp")
			return
		}
		where += " AND timestamp < ?"
		args = append(args, before)
	}
	args = append(args, limit+1)

	rows, err := rt.store.db.Query(
		"SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename FROM messages WHERE "+
			where+" ORDER BY timestamp DESC, id DESC LIMIT ?", args...)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	defer rows.Close()

	messages := make([]messageListItem, 0, limit+1)
	for rows.Next() {
		var item messageListItem
		var mediaType, filename sql.NullString
		if err := rows.Scan(&item.ID, &item.ChatJID, &item.Sender, &item.Content, &item.Timestamp, &item.IsFromMe, &mediaType, &filename); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		item.MediaType = mediaType.String
		item.Filename = filename.String
		messages = append(messages, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	writeEnvelope(w, http.StatusOK, struct {
		Messages []messageListItem `json:"messages"`
		HasMore  bool              `json:"has_more"`
	}{Messages: messages, HasMore: hasMore})
}

func (a *tenantAPI) handleTenantMedia(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.runtimeFor(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	messageID := r.PathValue("messageID")
	chatJID := strings.TrimSpace(r.URL.Query().Get("chat_jid"))
	if messageID == "" || chatJID == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "message id and chat_jid are required")
		return
	}

	mediaType, filename, _, _, _, _, _, err := rt.store.GetMediaInfo(messageID, chatJID)
	if err != nil || filename == "" {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Media message not found")
		return
	}

	baseDir, err := filepath.Abs(rt.store.MediaDir())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	localPath := filepath.Join(baseDir, sanitizeChatDir(chatJID), filepath.Base(filename))
	absPath, err := filepath.Abs(localPath)
	if err != nil || !strings.HasPrefix(absPath, baseDir+string(filepath.Separator)) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid media path")
		return
	}

	if _, statErr := os.Stat(absPath); errors.Is(statErr, os.ErrNotExist) {
		if rt.client == nil || rt.Status() != TenantConnected {
			writeAPIError(w, http.StatusServiceUnavailable, "TENANT_DISCONNECTED", "Tenant is not connected")
			return
		}
		success, _, _, downloaded, downloadErr := downloadMedia(rt.client, rt.store, rt.store.MediaDir(), messageID, chatJID)
		if !success || downloadErr != nil {
			message := "Failed to download media"
			if downloadErr != nil {
				message += ": " + downloadErr.Error()
			}
			writeAPIError(w, http.StatusInternalServerError, "DOWNLOAD_FAILED", message)
			return
		}
		absPath = downloaded
	}

	file, err := os.Open(absPath)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(absPath))
	if contentType == "" {
		switch mediaType {
		case "image":
			contentType = "image/jpeg"
		case "video":
			contentType = "video/mp4"
		case "audio":
			contentType = "audio/ogg"
		case "document":
			contentType = "application/octet-stream"
		default:
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filepath.Base(absPath)}))
	http.ServeContent(w, r, filepath.Base(absPath), info.ModTime(), file)
}

func parseSQLiteTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func dailyMessages(db *sql.DB) []dailyMessage {
	const days = 14
	counts := make(map[string]int, days)
	rows, err := db.Query(`
		SELECT date(timestamp) AS day, COUNT(*)
		FROM messages
		WHERE timestamp >= datetime('now', '-13 days')
		GROUP BY day`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var day string
			var count int
			if err := rows.Scan(&day, &count); err == nil {
				counts[day] = count
			}
		}
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	series := make([]dailyMessage, 0, days)
	for i := days - 1; i >= 0; i-- {
		date := today.AddDate(0, 0, -i).Format("2006-01-02")
		series = append(series, dailyMessage{Date: date, Count: counts[date]})
	}
	return series
}

func (a *tenantAPI) handleTenantStats(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.runtimeFor(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	stats := tenantStats{TenantID: rt.tenant.ID, Status: rt.Status()}
	var last sql.NullString
	if err := rt.store.db.QueryRow(`
		SELECT (SELECT COUNT(*) FROM chats),
		       (SELECT COUNT(*) FROM messages),
		       (SELECT COUNT(*) FROM events),
		       (SELECT MAX(timestamp) FROM messages)`).Scan(
		&stats.Chats, &stats.Messages, &stats.Events, &last); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if parsed, ok := parseSQLiteTime(last.String); ok {
		stats.LastMessageTime = &parsed
	}
	stats.MessagesByDay = dailyMessages(rt.store.db)
	if tenant, ok := a.m.tenants.Get(rt.tenant.ID); ok {
		for _, key := range tenant.Keys {
			if key.RevokedAt == nil {
				stats.Keys++
			}
		}
	}
	writeEnvelope(w, http.StatusOK, struct {
		Stats tenantStats `json:"stats"`
	}{Stats: stats})
}
