package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTenantKeyCannotReadAnotherTenant(t *testing.T) {
	manager, token := newTestManager(t)
	if _, _, _, err := manager.tenants.Create("tenant-b", "B"); err != nil {
		t.Fatal(err)
	}
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/tenant-b/status", token, "")

	if response.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d (%s)", response.Code, http.StatusForbidden, response.Body.String())
	}
	payload := decodeEnvelope(t, response)
	if payload.Error == nil || payload.Error.Code != "FORBIDDEN" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
}

func TestInvalidTenantIDIsRejectedBeforeLookup(t *testing.T) {
	manager, token := newTestManager(t)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/INVALID_ID/status", token, "")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d (%s)", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestCreateTenantRequiresConfiguredAdmin(t *testing.T) {
	manager, token := newTestManager(t)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodPost, "/api/v1/tenants", token, `{"id":"tenant-c","label":"C"}`)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d (%s)", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
	payload := decodeEnvelope(t, response)
	if payload.Error == nil || payload.Error.Code != "ADMIN_DISABLED" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
}

func TestTenantKeyLifecycle(t *testing.T) {
	store, err := NewTenantStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDefault("legacy-token"); err != nil {
		t.Fatal(err)
	}

	key, token, err := store.CreateKey(LegacyTenantID, "automation")
	if err != nil {
		t.Fatal(err)
	}
	p, ok := store.Authenticate(token)
	if !ok || p.IsAdmin || p.TenantID != LegacyTenantID || p.KeyID != key.ID {
		t.Fatalf("unexpected principal: %+v ok=%v", p, ok)
	}

	if err := store.RevokeKey(LegacyTenantID, key.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Authenticate(token); ok {
		t.Fatal("revoked key must not authenticate")
	}
	if _, ok := store.Authenticate("legacy-token"); !ok {
		t.Fatal("the legacy key must keep working")
	}
}

func TestLegacyV1SendRejectsInvalidBody(t *testing.T) {
	manager, token := newTestManager(t)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodPost, "/api/v1/send", token, `{"recipient":""}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusBadRequest)
	}
	payload := decodeEnvelope(t, response)
	if payload.Error == nil || payload.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
}

func TestSendRequiresConnectedTenant(t *testing.T) {
	manager, token := newTestManager(t)
	attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/send", token,
		`{"recipient":"5511999999999","message":"hi"}`)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d (%s)", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
	payload := decodeEnvelope(t, response)
	if payload.Error == nil || payload.Error.Code != "TENANT_DISCONNECTED" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
}

func TestAdminCanProvisionAndReadAnyTenant(t *testing.T) {
	manager, _ := newTestManagerWithAdmin(t, "admin-secret")
	if _, _, _, err := manager.tenants.Create("tenant-b", "B"); err != nil {
		t.Fatal(err)
	}
	handler := newTenantRESTHandler(manager)

	status := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/tenant-b/status", "admin-secret", "")
	if status.Code != http.StatusOK {
		t.Fatalf("admin status: got %d (%s)", status.Code, status.Body.String())
	}

	created := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/tenant-b/keys", "admin-secret", `{"label":"ci"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("admin key creation: got %d (%s)", created.Code, created.Body.String())
	}
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Key   tenantKeyView `json:"key"`
			Token string        `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(created.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success || payload.Data.Token == "" || payload.Data.Key.Label != "ci" {
		t.Fatalf("unexpected key payload: %+v", payload)
	}

	forbidden := doRequest(t, handler, http.MethodGet, "/api/v1/tenants", payload.Data.Token, "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("tenant key listing tenants: got %d, want %d", forbidden.Code, http.StatusForbidden)
	}

	listed := doRequest(t, handler, http.MethodGet, "/api/v1/tenants", "admin-secret", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("admin listing: got %d (%s)", listed.Code, listed.Body.String())
	}
}

func TestStoreMessageWritesEventAtomically(t *testing.T) {
	store, err := NewMessageStoreAt(filepath.Join(t.TempDir(), "messages.db"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	timestamp := time.Now().UTC().Truncate(time.Second)
	if err := store.StoreChat("5511@s.whatsapp.net", "Alice", timestamp); err != nil {
		t.Fatal(err)
	}
	meta := &MessageEventMeta{
		ChatName:    "Alice",
		SenderJID:   "5511@s.whatsapp.net",
		SenderName:  "Alice",
		MentionedMe: true,
	}
	msg := StoredMessage{
		ID:        "msg-1",
		ChatJID:   "5511@s.whatsapp.net",
		Sender:    "5511",
		Content:   "hello",
		Timestamp: timestamp,
	}
	seq, err := store.StoreMessage(msg, meta)
	if err != nil {
		t.Fatal(err)
	}
	if seq == 0 {
		t.Fatal("expected an event sequence")
	}

	var count int
	var eventSeq int64
	var chatName string
	var mentioned bool
	if err := store.db.QueryRow("SELECT COUNT(*) FROM events WHERE message_id = ? AND chat_jid = ?", msg.ID, msg.ChatJID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d events, want 1", count)
	}
	if err := store.db.QueryRow("SELECT seq, chat_name, mentioned_me FROM events WHERE message_id = ? AND chat_jid = ?", msg.ID, msg.ChatJID).Scan(&eventSeq, &chatName, &mentioned); err != nil {
		t.Fatal(err)
	}
	if eventSeq != seq || chatName != "Alice" || !mentioned {
		t.Fatalf("unexpected event: seq=%d chat=%q mentioned=%v", eventSeq, chatName, mentioned)
	}

	// A duplicate message keeps a single event and returns its sequence.
	msg.Content = "hello again"
	duplicateSeq, err := store.StoreMessage(msg, meta)
	if err != nil {
		t.Fatal(err)
	}
	if duplicateSeq != seq {
		t.Fatalf("duplicate returned seq %d, want %d", duplicateSeq, seq)
	}
	if err := store.db.QueryRow("SELECT COUNT(*) FROM events WHERE message_id = ? AND chat_jid = ?", msg.ID, msg.ChatJID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate wrote %d events, want 1", count)
	}

	// History sync (nil meta) must not create an event.
	history := StoredMessage{
		ID:        "msg-history",
		ChatJID:   "5511@s.whatsapp.net",
		Sender:    "5511",
		Content:   "old",
		Timestamp: timestamp,
	}
	if seq, err := store.StoreMessage(history, nil); err != nil || seq != 0 {
		t.Fatalf("history write: seq=%d err=%v", seq, err)
	}
	if err := store.db.QueryRow("SELECT COUNT(*) FROM events WHERE message_id = ?", history.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("history wrote %d events, want 0", count)
	}
}

func TestStoreMessageRollsBackOnForeignKeyFailure(t *testing.T) {
	store, err := NewMessageStoreAt(filepath.Join(t.TempDir(), "messages.db"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	msg := StoredMessage{
		ID:        "orphan",
		ChatJID:   "9999@s.whatsapp.net",
		Sender:    "9999",
		Content:   "no chat row",
		Timestamp: time.Now().UTC(),
	}
	meta := &MessageEventMeta{ChatName: "Ghost"}
	if _, err := store.StoreMessage(msg, meta); err == nil {
		t.Fatal("expected a foreign key failure")
	}

	for _, table := range []string{"messages", "events"} {
		var count int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE chat_jid = ?", msg.ChatJID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s has %d rows after rollback, want 0", table, count)
		}
	}
}

func TestTenantPathsAreIsolated(t *testing.T) {
	root := t.TempDir()
	store, err := NewTenantStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDefault("legacy-token"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := store.Create("tenant-b", "B"); err != nil {
		t.Fatal(err)
	}
	manager := NewTenantManager(store, nil)
	defer manager.StopAll()

	defaultTenant, _ := store.Get(LegacyTenantID)
	defaultRT, err := manager.newRuntime(defaultTenant)
	if err != nil {
		t.Fatal(err)
	}
	defer defaultRT.store.Close()
	tenantB, _ := store.Get("tenant-b")
	tenantBRT, err := manager.newRuntime(tenantB)
	if err != nil {
		t.Fatal(err)
	}
	defer tenantBRT.store.Close()

	if _, err := os.Stat(filepath.Join(root, "messages.db")); err != nil {
		t.Fatalf("default messages.db missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tenants", "tenant-b", "messages.db")); err != nil {
		t.Fatalf("tenant messages.db missing: %v", err)
	}
	if defaultRT.store.MediaDir() != root {
		t.Fatalf("default media dir = %q, want %q", defaultRT.store.MediaDir(), root)
	}
	if want := filepath.Join(root, "tenants", "tenant-b", "media"); tenantBRT.store.MediaDir() != want {
		t.Fatalf("tenant media dir = %q, want %q", tenantBRT.store.MediaDir(), want)
	}
}

func TestTenantStorePersistsAndDeletes(t *testing.T) {
	root := t.TempDir()
	store, err := NewTenantStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDefault("legacy-token"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := store.Create("tenant-b", "B"); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewTenantStore(root)
	if err != nil {
		t.Fatal(err)
	}
	views := reloaded.List()
	if len(views) != 2 {
		t.Fatalf("got %d tenants, want 2", len(views))
	}
	if views[0].ID != LegacyTenantID || views[1].ID != "tenant-b" {
		t.Fatalf("unexpected order: %+v", views)
	}
	if err := reloaded.Delete("tenant-b"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.Get("tenant-b"); ok {
		t.Fatal("deleted tenant still present")
	}
	if err := reloaded.EnsureDefault("legacy-token"); err != nil {
		t.Fatal(err)
	}
}
