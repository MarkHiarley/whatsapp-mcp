package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type chatsPayload struct {
	Success bool `json:"success"`
	Data    struct {
		Chats []chatListItem `json:"chats"`
	} `json:"data"`
}

type messagesPayload struct {
	Success bool `json:"success"`
	Data    struct {
		Messages []messageListItem `json:"messages"`
		HasMore  bool              `json:"has_more"`
	} `json:"data"`
}

type statsPayload struct {
	Success bool `json:"success"`
	Data    struct {
		Stats tenantStats `json:"stats"`
	} `json:"data"`
}

func seedChat(t *testing.T, rt *tenantRuntime, chatJID, name, messageID, content string, at time.Time) {
	t.Helper()
	if err := rt.store.StoreChat(chatJID, name, at); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.StoreMessage(StoredMessage{
		ID:        messageID,
		ChatJID:   chatJID,
		Sender:    "5511999999999",
		Content:   content,
		Timestamp: at,
	}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestPanelChatsAndMessages(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantConnected)
	at := time.Now().UTC().Truncate(time.Second)
	seedChat(t, rt, "5511999999999@s.whatsapp.net", "Ana", "m1", "primeira", at)

	handler := newTenantRESTHandler(manager)
	response := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/chats", token, "")
	if response.Code != http.StatusOK {
		t.Fatalf("chats status = %d (%s)", response.Code, response.Body.String())
	}
	var chats chatsPayload
	if err := json.NewDecoder(response.Body).Decode(&chats); err != nil {
		t.Fatal(err)
	}
	if len(chats.Data.Chats) != 1 {
		t.Fatalf("got %d chats, want 1", len(chats.Data.Chats))
	}
	chat := chats.Data.Chats[0]
	if chat.Name != "Ana" || chat.LastMessage != "primeira" || chat.IsGroup {
		t.Fatalf("unexpected chat: %+v", chat)
	}

	response = doRequest(t, handler, http.MethodGet,
		"/api/v1/tenants/default/chats/5511999999999@s.whatsapp.net/messages", token, "")
	if response.Code != http.StatusOK {
		t.Fatalf("messages status = %d (%s)", response.Code, response.Body.String())
	}
	var messages messagesPayload
	if err := json.NewDecoder(response.Body).Decode(&messages); err != nil {
		t.Fatal(err)
	}
	if len(messages.Data.Messages) != 1 || messages.Data.Messages[0].Content != "primeira" {
		t.Fatalf("unexpected messages: %+v", messages.Data)
	}
	if messages.Data.HasMore {
		t.Fatal("has_more should be false")
	}
}

func TestPanelChatsFilter(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantConnected)
	at := time.Now().UTC()
	seedChat(t, rt, "5511999999999@s.whatsapp.net", "Ana", "m1", "oi", at)
	seedChat(t, rt, "5511888888888@s.whatsapp.net", "Bruno", "m2", "oi", at)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/chats?query=Bruno", token, "")
	var chats chatsPayload
	if err := json.NewDecoder(response.Body).Decode(&chats); err != nil {
		t.Fatal(err)
	}
	if len(chats.Data.Chats) != 1 || chats.Data.Chats[0].Name != "Bruno" {
		t.Fatalf("unexpected filtered chats: %+v", chats.Data.Chats)
	}
}

func TestPanelMessagesBeforeAndPagination(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantConnected)
	base := time.Now().UTC().Truncate(time.Second)
	seedChat(t, rt, "5511999999999@s.whatsapp.net", "Ana", "m1", "um", base.Add(-2*time.Minute))
	if err := rt.store.StoreChat("5511999999999@s.whatsapp.net", "Ana", base); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.StoreMessage(StoredMessage{ID: "m2", ChatJID: "5511999999999@s.whatsapp.net", Sender: "5511999999999", Content: "dois", Timestamp: base}, nil); err != nil {
		t.Fatal(err)
	}
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet,
		"/api/v1/tenants/default/chats/5511999999999@s.whatsapp.net/messages?limit=1", token, "")
	var page messagesPayload
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data.Messages) != 1 || page.Data.Messages[0].ID != "m2" || !page.Data.HasMore {
		t.Fatalf("unexpected first page: %+v", page.Data)
	}

	before := base.Format(time.RFC3339)
	response = doRequest(t, handler, http.MethodGet,
		"/api/v1/tenants/default/chats/5511999999999@s.whatsapp.net/messages?before="+before, token, "")
	var older messagesPayload
	if err := json.NewDecoder(response.Body).Decode(&older); err != nil {
		t.Fatal(err)
	}
	if len(older.Data.Messages) != 1 || older.Data.Messages[0].ID != "m1" {
		t.Fatalf("unexpected older page: %+v", older.Data)
	}

	response = doRequest(t, handler, http.MethodGet,
		"/api/v1/tenants/default/chats/5511999999999@s.whatsapp.net/messages?before=not-a-date", token, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid before status = %d", response.Code)
	}
}

func TestPanelStats(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantConnected)
	at := time.Now().UTC()
	seedChat(t, rt, "5511999999999@s.whatsapp.net", "Ana", "m1", "um", at)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/stats", token, "")
	if response.Code != http.StatusOK {
		t.Fatalf("stats status = %d (%s)", response.Code, response.Body.String())
	}
	var payload statsPayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	stats := payload.Data.Stats
	if stats.Chats != 1 || stats.Messages != 1 || stats.Keys != 1 || stats.Status != TenantConnected {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestPanelReadRejectsOtherTenant(t *testing.T) {
	manager, token := newTestManager(t)
	if _, _, _, err := manager.tenants.Create("tenant-b", "B"); err != nil {
		t.Fatal(err)
	}
	attachTestRuntime(t, manager, "tenant-b", TenantConnected)
	handler := newTenantRESTHandler(manager)

	for _, path := range []string{
		"/api/v1/tenants/tenant-b/chats",
		"/api/v1/tenants/tenant-b/stats",
	} {
		response := doRequest(t, handler, http.MethodGet, path, token, "")
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d, want 403", path, response.Code)
		}
	}
}

func TestPanelMediaServesLocalFile(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	at := time.Now().UTC()
	chatJID := "5511999999999@s.whatsapp.net"
	if err := rt.store.StoreChat(chatJID, "Ana", at); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.StoreMessage(StoredMessage{
		ID: "media-1", ChatJID: chatJID, Sender: "5511999999999",
		Timestamp: at, MediaType: "image", Filename: "foto.jpg",
	}, nil); err != nil {
		t.Fatal(err)
	}
	chatDir := filepath.Join(rt.store.MediaDir(), sanitizeChatDir(chatJID))
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		t.Fatal(err)
	}
	want := []byte("jpeg-bytes")
	if err := os.WriteFile(filepath.Join(chatDir, "foto.jpg"), want, 0644); err != nil {
		t.Fatal(err)
	}
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet,
		"/api/v1/tenants/default/media/media-1?chat_jid="+chatJID, token, "")
	if response.Code != http.StatusOK {
		t.Fatalf("media status = %d (%s)", response.Code, response.Body.String())
	}
	if got := response.Body.Bytes(); string(got) != string(want) {
		t.Fatalf("media body = %q, want %q", got, want)
	}
	if ct := response.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("content type = %q", ct)
	}
}

func TestPanelMediaMissingNeedsConnection(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	at := time.Now().UTC()
	chatJID := "5511999999999@s.whatsapp.net"
	if err := rt.store.StoreChat(chatJID, "Ana", at); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.StoreMessage(StoredMessage{
		ID: "media-2", ChatJID: chatJID, Sender: "5511999999999",
		Timestamp: at, MediaType: "image", Filename: "nao-existe.jpg",
	}, nil); err != nil {
		t.Fatal(err)
	}
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet,
		"/api/v1/tenants/default/media/media-2?chat_jid="+chatJID, token, "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing media status = %d, want 503", response.Code)
	}
}

func TestPanelMediaRequiresChatJID(t *testing.T) {
	manager, token := newTestManager(t)
	attachTestRuntime(t, manager, LegacyTenantID, TenantConnected)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/media/media-1", token, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}
