package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func createWebhookViaAPI(t *testing.T, handler http.Handler, token, tenant, target, ruleID string) (webhookView, string) {
	t.Helper()
	body, err := json.Marshal(createWebhookRequest{URL: target, RuleID: ruleID})
	if err != nil {
		t.Fatal(err)
	}
	response := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/"+tenant+"/webhooks", token, string(body))
	if response.Code != http.StatusCreated {
		t.Fatalf("create webhook: got %d (%s)", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Webhook webhookView `json:"webhook"`
			Secret  string      `json:"secret"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data.Webhook, payload.Data.Secret
}

func forceDeliveryDue(t *testing.T, store *MessageStore, webhookID string, seq int64) {
	t.Helper()
	_, err := store.db.Exec(`
		UPDATE event_deliveries SET next_attempt_at = ?
		WHERE webhook_id = ? AND seq = ?
	`, time.Now().UTC().Add(-time.Minute), webhookID, seq)
	if err != nil {
		t.Fatal(err)
	}
}

func expectedSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookURLRejectsPrivateAddressesUnlessAllowed(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)
	createRuleViaAPI(t, handler, token, LegacyTenantID, "r", RuleMatch{TextContains: []string{"x"}})

	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "0")
	blocked := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/webhooks", token,
		`{"url":"http://127.0.0.1:9/hook","rule_id":"r"}`)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("private url: got %d (%s)", blocked.Code, blocked.Body.String())
	}
	if payload := decodeEnvelope(t, blocked); payload.Error == nil || payload.Error.Code != "WEBHOOK_INVALID" {
		t.Fatalf("unexpected private url payload: %+v", payload.Error)
	}

	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	allowed, secret := createWebhookViaAPI(t, handler, token, LegacyTenantID, "http://127.0.0.1:9/hook", "r")
	if allowed.ID == "" || len(secret) != 64 {
		t.Fatalf("unexpected webhook: %+v secret=%q", allowed, secret)
	}

	userinfo := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/webhooks", token,
		`{"url":"http://user:pass@127.0.0.1:9/hook","rule_id":"r"}`)
	if userinfo.Code != http.StatusBadRequest {
		t.Fatalf("userinfo url: got %d (%s)", userinfo.Code, userinfo.Body.String())
	}

	scheme := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/webhooks", token,
		`{"url":"file:///etc/passwd","rule_id":"r"}`)
	if scheme.Code != http.StatusBadRequest {
		t.Fatalf("file scheme: got %d (%s)", scheme.Code, scheme.Body.String())
	}

	missingRule := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/webhooks", token,
		`{"url":"http://127.0.0.1:9/hook","rule_id":"nope"}`)
	if missingRule.Code != http.StatusNotFound {
		t.Fatalf("missing rule: got %d (%s)", missingRule.Code, missingRule.Body.String())
	}

	rotated := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/webhooks/"+allowed.ID+"/secret", token, "")
	if rotated.Code != http.StatusOK {
		t.Fatalf("rotate secret: got %d (%s)", rotated.Code, rotated.Body.String())
	}
	var payload struct {
		Data struct {
			Secret string `json:"secret"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rotated.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Secret) != 64 || payload.Data.Secret == secret {
		t.Fatalf("unexpected rotated secret %q", payload.Data.Secret)
	}
	stored, ok, err := rt.store.getWebhook(allowed.ID)
	if err != nil || !ok {
		t.Fatalf("load webhook: ok=%v err=%v", ok, err)
	}
	if stored.Secret != payload.Data.Secret {
		t.Fatal("rotated secret was not persisted")
	}
}

func TestWebhookWorkerDeliversSignedEvent(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "urgentes", RuleMatch{TextContains: []string{"urgente"}})

	type received struct {
		body        []byte
		eventID     string
		signature   string
		contentType string
	}
	var (
		mu      sync.Mutex
		private []received
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		private = append(private, received{
			body:        body,
			eventID:     r.Header.Get("X-WhatsApp-Event-Id"),
			signature:   r.Header.Get("X-WhatsApp-Signature"),
			contentType: r.Header.Get("Content-Type"),
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook, secret := createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "urgentes")
	seq := seedEvent(t, rt.store, "m1", "5511@s.whatsapp.net", "isso é urgente", false)

	processed, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected the worker to process the matching event")
	}

	mu.Lock()
	delivered := append([]received(nil), private...)
	mu.Unlock()
	if len(delivered) != 1 {
		t.Fatalf("server received %d deliveries, want 1", len(delivered))
	}
	got := delivered[0]
	if got.contentType != "application/json" {
		t.Fatalf("unexpected content type %q", got.contentType)
	}
	if got.eventID != "default:"+strconv.FormatInt(seq, 10) {
		t.Fatalf("unexpected event id %q", got.eventID)
	}
	if got.signature != expectedSignature(secret, got.body) {
		t.Fatalf("signature mismatch: %q", got.signature)
	}
	var event Event
	if err := json.Unmarshal(got.body, &event); err != nil {
		t.Fatal(err)
	}
	if event.Seq != seq || event.Content != "isso é urgente" {
		t.Fatalf("unexpected event body: %+v", event)
	}

	state, exists, err := rt.store.deliveryFor(seq, webhook.ID)
	if err != nil || !exists {
		t.Fatalf("delivery row: exists=%v err=%v", exists, err)
	}
	if state.DeliveredAt == nil || state.Attempts != 1 || state.LastStatus == nil || *state.LastStatus != 200 {
		t.Fatalf("unexpected delivery state: %+v", state)
	}
	cursor, err := rt.store.consumerCursor(webhookConsumerID(webhook.ID), "urgentes")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != seq {
		t.Fatalf("cursor = %d, want %d", cursor, seq)
	}

	again, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("worker should be idle once the queue is drained")
	}

	log := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/webhooks/"+webhook.ID+"/deliveries", token, "")
	if log.Code != http.StatusOK {
		t.Fatalf("deliveries: got %d (%s)", log.Code, log.Body.String())
	}
	var logPayload struct {
		Data struct {
			Deliveries []DeliveryView `json:"deliveries"`
		} `json:"data"`
	}
	if err := json.NewDecoder(log.Body).Decode(&logPayload); err != nil {
		t.Fatal(err)
	}
	if len(logPayload.Data.Deliveries) != 1 || logPayload.Data.Deliveries[0].DeliveredAt == nil {
		t.Fatalf("unexpected delivery log: %+v", logPayload.Data.Deliveries)
	}
}

func TestWebhookRetryThenSuccess(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r", RuleMatch{TextContains: []string{"urgente"}})

	var (
		mu    sync.Mutex
		calls int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		current := calls
		mu.Unlock()
		if current < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook, _ := createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "r")
	seq := seedEvent(t, rt.store, "m1", "5511@s.whatsapp.net", "urgente", false)

	if _, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID); err != nil {
		t.Fatal(err)
	}
	state, _, err := rt.store.deliveryFor(seq, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Attempts != 1 || state.NextAttemptAt == nil || state.DeliveredAt != nil {
		t.Fatalf("unexpected first failure state: %+v", state)
	}

	if processed, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID); err != nil || processed {
		t.Fatalf("backoff should block the second call: processed=%v err=%v", processed, err)
	}

	forceDeliveryDue(t, rt.store, webhook.ID, seq)
	if _, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID); err != nil {
		t.Fatal(err)
	}
	state, _, err = rt.store.deliveryFor(seq, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Attempts != 2 || state.DeliveredAt != nil {
		t.Fatalf("unexpected second failure state: %+v", state)
	}

	forceDeliveryDue(t, rt.store, webhook.ID, seq)
	if _, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID); err != nil {
		t.Fatal(err)
	}
	state, _, err = rt.store.deliveryFor(seq, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Attempts != 3 || state.DeliveredAt == nil || state.LastStatus == nil || *state.LastStatus != 200 {
		t.Fatalf("unexpected success state: %+v", state)
	}
	cursor, err := rt.store.consumerCursor(webhookConsumerID(webhook.ID), "r")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != seq {
		t.Fatalf("cursor = %d, want %d", cursor, seq)
	}

	mu.Lock()
	total := calls
	mu.Unlock()
	if total != 3 {
		t.Fatalf("server saw %d calls, want 3", total)
	}
}

func TestWebhookDeadLetteredAfterMaxAttempts(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r", RuleMatch{TextContains: []string{"urgente"}})

	var (
		mu    sync.Mutex
		calls int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	webhook, _ := createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "r")
	seq := seedEvent(t, rt.store, "m1", "5511@s.whatsapp.net", "urgente", false)

	for attempt := 0; attempt < webhookDeliveryAttemptsLimit; attempt++ {
		if attempt > 0 {
			forceDeliveryDue(t, rt.store, webhook.ID, seq)
		}
		if _, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID); err != nil {
			t.Fatal(err)
		}
	}

	state, exists, err := rt.store.deliveryFor(seq, webhook.ID)
	if err != nil || !exists {
		t.Fatalf("delivery row: exists=%v err=%v", exists, err)
	}
	if state.Attempts != webhookDeliveryAttemptsLimit {
		t.Fatalf("attempts = %d, want %d", state.Attempts, webhookDeliveryAttemptsLimit)
	}
	if !strings.HasPrefix(state.LastError, "dead:") {
		t.Fatalf("expected dead letter, got %q", state.LastError)
	}
	if state.DeliveredAt != nil {
		t.Fatal("dead letter must not be delivered")
	}
	cursor, err := rt.store.consumerCursor(webhookConsumerID(webhook.ID), "r")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != seq {
		t.Fatalf("cursor = %d, want %d", cursor, seq)
	}

	if processed, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID); err != nil || processed {
		t.Fatalf("dead letter should not be retried: processed=%v err=%v", processed, err)
	}
	mu.Lock()
	total := calls
	mu.Unlock()
	if total != webhookDeliveryAttemptsLimit {
		t.Fatalf("server saw %d calls, want %d", total, webhookDeliveryAttemptsLimit)
	}

	log := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/webhooks/"+webhook.ID+"/deliveries", token, "")
	if log.Code != http.StatusOK {
		t.Fatalf("deliveries: got %d (%s)", log.Code, log.Body.String())
	}
	var logPayload struct {
		Data struct {
			Deliveries []DeliveryView `json:"deliveries"`
		} `json:"data"`
	}
	if err := json.NewDecoder(log.Body).Decode(&logPayload); err != nil {
		t.Fatal(err)
	}
	if len(logPayload.Data.Deliveries) != 1 || !strings.HasPrefix(logPayload.Data.Deliveries[0].LastError, "dead:") {
		t.Fatalf("unexpected delivery log: %+v", logPayload.Data.Deliveries)
	}
}

func TestWebhookSkipsNonMatchingEvents(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r", RuleMatch{TextContains: []string{"nunca"}})

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook, _ := createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "r")
	seq := seedEvent(t, rt.store, "m1", "5511@s.whatsapp.net", "hello", false)

	processed, err := manager.processWebhookOnce(context.Background(), rt, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("cursor should advance over the non-matching event")
	}
	if _, exists, err := rt.store.deliveryFor(seq, webhook.ID); err != nil || exists {
		t.Fatalf("delivery row: exists=%v err=%v", exists, err)
	}
	cursor, err := rt.store.consumerCursor(webhookConsumerID(webhook.ID), "r")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != seq {
		t.Fatalf("cursor = %d, want %d", cursor, seq)
	}
	if calls != 0 {
		t.Fatalf("server received %d calls, want 0", calls)
	}
}

func TestWebhookIsolationBetweenTenants(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rtA := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	rtB := attachTestRuntime(t, manager, "tenant-b", TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r", RuleMatch{TextContains: []string{"urgente"}})

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook, _ := createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "r")
	seedEvent(t, rtB.store, "b1", "5511@s.whatsapp.net", "urgente no tenant B", false)

	processed, err := manager.processWebhookOnce(context.Background(), rtA, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if processed || calls != 0 {
		t.Fatalf("tenant A must not deliver tenant B events: processed=%v calls=%d", processed, calls)
	}

	seqA := seedEvent(t, rtA.store, "a1", "5511@s.whatsapp.net", "urgente no tenant A", false)
	processed, err = manager.processWebhookOnce(context.Background(), rtA, webhook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !processed || calls != 1 {
		t.Fatalf("tenant A event was not delivered: processed=%v calls=%d", processed, calls)
	}
	cursor, err := rtA.store.consumerCursor(webhookConsumerID(webhook.ID), "r")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != seqA {
		t.Fatalf("cursor = %d, want %d", cursor, seqA)
	}
	if count := countEvents(t, rtB.store); count != 1 {
		t.Fatalf("tenant B event count = %d, want 1", count)
	}
}

func countEvents(t *testing.T, store *MessageStore) int {
	t.Helper()
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
