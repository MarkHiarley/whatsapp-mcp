package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWebhookUpdateAndDeleteHTTP(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r1", RuleMatch{TextContains: []string{"a"}})
	createRuleViaAPI(t, handler, token, LegacyTenantID, "r2", RuleMatch{TextContains: []string{"b"}})
	webhook, _ := createWebhookViaAPI(t, handler, token, LegacyTenantID, "http://127.0.0.1:9/hook", "r1")

	updated := doRequest(t, handler, http.MethodPatch,
		"/api/v1/tenants/default/webhooks/"+webhook.ID,
		token, `{"rule_id":"r2","enabled":false,"headers":{"X-Test":"1"}}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update webhook: got %d (%s)", updated.Code, updated.Body.String())
	}
	stored, ok, err := rt.store.getWebhook(webhook.ID)
	if err != nil || !ok {
		t.Fatalf("load updated webhook: ok=%v err=%v", ok, err)
	}
	if stored.RuleID != "r2" || stored.Enabled || stored.Headers["X-Test"] != "1" {
		t.Fatalf("unexpected updated webhook: %+v", stored)
	}

	empty := doRequest(t, handler, http.MethodPatch,
		"/api/v1/tenants/default/webhooks/"+webhook.ID, token, `{}`)
	if empty.Code != http.StatusBadRequest {
		t.Fatalf("empty update: got %d (%s)", empty.Code, empty.Body.String())
	}

	missingRule := doRequest(t, handler, http.MethodPatch,
		"/api/v1/tenants/default/webhooks/"+webhook.ID, token, `{"rule_id":"nope"}`)
	if missingRule.Code != http.StatusNotFound {
		t.Fatalf("missing rule update: got %d (%s)", missingRule.Code, missingRule.Body.String())
	}

	badHeader := doRequest(t, handler, http.MethodPatch,
		"/api/v1/tenants/default/webhooks/"+webhook.ID, token,
		`{"headers":{"Host":"evil.example"}}`)
	if badHeader.Code != http.StatusBadRequest {
		t.Fatalf("reserved header update: got %d (%s)", badHeader.Code, badHeader.Body.String())
	}

	method := doRequest(t, handler, http.MethodPut,
		"/api/v1/tenants/default/webhooks/"+webhook.ID, token, "")
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT webhook: got %d (%s)", method.Code, method.Body.String())
	}

	deleted := doRequest(t, handler, http.MethodDelete,
		"/api/v1/tenants/default/webhooks/"+webhook.ID, token, "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete webhook: got %d (%s)", deleted.Code, deleted.Body.String())
	}
	var payload struct {
		Data struct {
			DeletedID string `json:"deleted_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(deleted.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.DeletedID != webhook.ID {
		t.Fatalf("deleted_id = %q, want %q", payload.Data.DeletedID, webhook.ID)
	}

	gone := doRequest(t, handler, http.MethodGet,
		"/api/v1/tenants/default/webhooks/"+webhook.ID, token, "")
	if gone.Code != http.StatusNotFound {
		t.Fatalf("deleted webhook: got %d (%s)", gone.Code, gone.Body.String())
	}
}

func TestWebhookLiveMethodsRejectUnknownWebhook(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPatch, "/api/v1/tenants/default/webhooks/missing"},
		{http.MethodPost, "/api/v1/tenants/default/webhooks/missing/secret"},
		{http.MethodGet, "/api/v1/tenants/default/webhooks/missing/deliveries"},
	} {
		response := doRequest(t, handler, tc.method, tc.path, token, "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s %s: got %d (%s)", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
}
