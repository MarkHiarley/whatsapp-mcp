package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// seedEvent inserts one event directly, returning its sequence.
func seedEvent(t *testing.T, store *MessageStore, messageID, chatJID, content string, isGroup bool) int64 {
	t.Helper()
	timestamp := time.Now().UTC().Truncate(time.Second)
	result, err := store.db.Exec(`
		INSERT INTO events
			(message_id, chat_jid, chat_name, sender_jid, sender_name, is_group, is_from_me,
			 mentioned_me, content, media_type, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, messageID, chatJID, "Test", "5511@s.whatsapp.net", "Alice", isGroup, false, false,
		content, "", timestamp, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

func createRuleViaAPI(t *testing.T, handler http.Handler, token, tenant, id string, match RuleMatch) EventRule {
	t.Helper()
	body, err := json.Marshal(createRuleRequest{ID: id, Name: id, Match: match})
	if err != nil {
		t.Fatal(err)
	}
	response := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/"+tenant+"/rules", token, string(body))
	if response.Code != http.StatusCreated {
		t.Fatalf("create rule %s: got %d (%s)", id, response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Rule EventRule `json:"rule"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data.Rule
}

func TestRuleLifecycleHTTP(t *testing.T) {
	manager, token := newTestManager(t)
	attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	if _, _, _, err := manager.tenants.Create("tenant-b", "B"); err != nil {
		t.Fatal(err)
	}
	handler := newTenantRESTHandler(manager)

	rule := createRuleViaAPI(t, handler, token, LegacyTenantID, "urgentes", RuleMatch{
		IsGroup:      boolPtr(true),
		TextContains: []string{"urgente"},
	})
	if !rule.Enabled || rule.ID != "urgentes" || rule.Name != "urgentes" {
		t.Fatalf("unexpected created rule: %+v", rule)
	}

	listed := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/rules", token, "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list rules: got %d (%s)", listed.Code, listed.Body.String())
	}
	var listPayload struct {
		Data struct {
			Rules []EventRule `json:"rules"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listed.Body).Decode(&listPayload); err != nil {
		t.Fatal(err)
	}
	if len(listPayload.Data.Rules) != 1 || listPayload.Data.Rules[0].ID != "urgentes" {
		t.Fatalf("unexpected rule list: %+v", listPayload.Data.Rules)
	}

	patched := doRequest(t, handler, http.MethodPatch, "/api/v1/tenants/default/rules/urgentes", token, `{"enabled":false}`)
	if patched.Code != http.StatusOK {
		t.Fatalf("patch rule: got %d (%s)", patched.Code, patched.Body.String())
	}
	var patchPayload struct {
		Data struct {
			Rule EventRule `json:"rule"`
		} `json:"data"`
	}
	if err := json.NewDecoder(patched.Body).Decode(&patchPayload); err != nil {
		t.Fatal(err)
	}
	if patchPayload.Data.Rule.Enabled {
		t.Fatal("rule should be disabled after patch")
	}

	invalid := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/rules", token,
		`{"name":"bad","match":{"text_regex":"["}}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid regex: got %d (%s)", invalid.Code, invalid.Body.String())
	}
	if payload := decodeEnvelope(t, invalid); payload.Error == nil || payload.Error.Code != "RULE_INVALID" {
		t.Fatalf("unexpected invalid regex payload: %+v", payload.Error)
	}

	emptyMatch := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/rules", token,
		`{"name":"empty","match":{"text_contains":[""]}}`)
	if emptyMatch.Code != http.StatusBadRequest {
		t.Fatalf("empty text_contains: got %d (%s)", emptyMatch.Code, emptyMatch.Body.String())
	}

	duplicate := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/rules", token,
		`{"id":"urgentes","name":"dup","match":{"is_group":true}}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate rule: got %d (%s)", duplicate.Code, duplicate.Body.String())
	}

	crossTenant := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/tenant-b/rules", token, "")
	if crossTenant.Code != http.StatusForbidden {
		t.Fatalf("cross tenant rules: got %d (%s)", crossTenant.Code, crossTenant.Body.String())
	}

	deleted := doRequest(t, handler, http.MethodDelete, "/api/v1/tenants/default/rules/urgentes", token, "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete rule: got %d (%s)", deleted.Code, deleted.Body.String())
	}
	missing := doRequest(t, handler, http.MethodDelete, "/api/v1/tenants/default/rules/urgentes", token, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("delete missing rule: got %d (%s)", missing.Code, missing.Body.String())
	}
}

func TestListEventsFilteredByRuleAdvancesCursor(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	seq1 := seedEvent(t, rt.store, "m1", "5511@s.whatsapp.net", "hello", false)
	seq2 := seedEvent(t, rt.store, "m2", "120363@g.us", "isso é urgente", true)
	seq3 := seedEvent(t, rt.store, "m3", "5512@s.whatsapp.net", "random", false)
	if !(seq1 < seq2 && seq2 < seq3) {
		t.Fatalf("unexpected sequences: %d %d %d", seq1, seq2, seq3)
	}

	createRuleViaAPI(t, handler, token, LegacyTenantID, "grupo", RuleMatch{IsGroup: boolPtr(true)})

	response := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/events?rule=grupo&limit=10", token, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list events: got %d (%s)", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Events    []Event `json:"events"`
			NextAfter int64   `json:"next_after"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Events) != 1 || payload.Data.Events[0].Seq != seq2 {
		t.Fatalf("unexpected filtered events: %+v", payload.Data.Events)
	}
	if payload.Data.NextAfter != seq3 {
		t.Fatalf("next_after = %d, want %d", payload.Data.NextAfter, seq3)
	}

	partial := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/events?rule=grupo&after=0&limit=1", token, "")
	if partial.Code != http.StatusOK {
		t.Fatalf("partial events: got %d (%s)", partial.Code, partial.Body.String())
	}
	if err := json.NewDecoder(partial.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Events) != 1 || payload.Data.Events[0].Seq != seq2 || payload.Data.NextAfter != seq2 {
		t.Fatalf("unexpected partial page: %+v next=%d", payload.Data.Events, payload.Data.NextAfter)
	}

	unfiltered := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/events?limit=2", token, "")
	if unfiltered.Code != http.StatusOK {
		t.Fatalf("unfiltered events: got %d (%s)", unfiltered.Code, unfiltered.Body.String())
	}
	if err := json.NewDecoder(unfiltered.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Events) != 2 || payload.Data.NextAfter != seq2 {
		t.Fatalf("unexpected unfiltered page: %+v next=%d", payload.Data.Events, payload.Data.NextAfter)
	}

	missing := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/events?rule=nope", token, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing rule filter: got %d (%s)", missing.Code, missing.Body.String())
	}

	badAfter := doRequest(t, handler, http.MethodGet, "/api/v1/tenants/default/events?after=-1", token, "")
	if badAfter.Code != http.StatusBadRequest {
		t.Fatalf("negative after: got %d (%s)", badAfter.Code, badAfter.Body.String())
	}
}

func TestConsumerAckIsMonotonic(t *testing.T) {
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	first := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/consumers/crm/ack", token,
		`{"rule_id":"r1","seq":5}`)
	if first.Code != http.StatusOK {
		t.Fatalf("ack: got %d (%s)", first.Code, first.Body.String())
	}
	var payload struct {
		Data struct {
			ConsumerID string `json:"consumer_id"`
			RuleID     string `json:"rule_id"`
			LastSeq    int64  `json:"last_seq"`
		} `json:"data"`
	}
	if err := json.NewDecoder(first.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.ConsumerID != "crm" || payload.Data.RuleID != "r1" || payload.Data.LastSeq != 5 {
		t.Fatalf("unexpected ack: %+v", payload.Data)
	}

	regressed := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/consumers/crm/ack", token,
		`{"rule_id":"r1","seq":3}`)
	if regressed.Code != http.StatusOK {
		t.Fatalf("ack regression: got %d (%s)", regressed.Code, regressed.Body.String())
	}
	if err := json.NewDecoder(regressed.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.LastSeq != 5 {
		t.Fatalf("cursor regressed to %d", payload.Data.LastSeq)
	}

	cursor, err := rt.store.consumerCursor("crm", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 5 {
		t.Fatalf("stored cursor = %d, want 5", cursor)
	}

	invalidConsumer := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/consumers/-bad/ack", token,
		`{"rule_id":"r1","seq":1}`)
	if invalidConsumer.Code != http.StatusBadRequest {
		t.Fatalf("invalid consumer: got %d (%s)", invalidConsumer.Code, invalidConsumer.Body.String())
	}

	negative := doRequest(t, handler, http.MethodPost, "/api/v1/tenants/default/consumers/crm/ack", token,
		`{"rule_id":"r1","seq":-1}`)
	if negative.Code != http.StatusBadRequest {
		t.Fatalf("negative seq: got %d (%s)", negative.Code, negative.Body.String())
	}
}
