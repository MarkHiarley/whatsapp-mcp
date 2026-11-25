package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestProcessAllWebhooksDrainsEnabledTenants verifies the worker fan-out:
// every enabled webhook of every tenant is advanced, and disabled webhooks
// stay inert until re-enabled.
func TestProcessAllWebhooksDrainsEnabledTenants(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, _ := newTestManagerWithAdmin(t, "admin-test-token")
	token := "admin-test-token"
	rtA := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	rtB := attachTestRuntime(t, manager, "tenant-b", TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r-a", RuleMatch{TextContains: []string{"urgente"}})
	createRuleViaAPI(t, handler, token, "tenant-b", "r-b", RuleMatch{TextContains: []string{"urgente"}})

	var deliveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		deliveries.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookA, _ := createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "r-a")
	webhookB, _ := createWebhookViaAPI(t, handler, token, "tenant-b", server.URL, "r-b")

	// Disable tenant B's webhook without going through HTTP so the fan-out
	// sees the same state the delivery worker would see.
	storedB, ok, err := rtB.store.getWebhook(webhookB.ID)
	if err != nil || !ok {
		t.Fatalf("load webhook B: ok=%v err=%v", ok, err)
	}
	storedB.Enabled = false
	if _, err := rtB.store.updateWebhook(storedB); err != nil {
		t.Fatal(err)
	}

	seqA := seedEvent(t, rtA.store, "a1", "5511@s.whatsapp.net", "urgente A", false)
	seqB := seedEvent(t, rtB.store, "b1", "5511@s.whatsapp.net", "urgente B", false)

	manager.processAllWebhooks(context.Background())

	if got := deliveries.Load(); got != 1 {
		t.Fatalf("deliveries = %d, want only tenant A (1)", got)
	}
	if cursor := mustCursor(t, rtA.store, webhookA.ID, "r-a"); cursor != seqA {
		t.Fatalf("tenant A cursor = %d, want %d", cursor, seqA)
	}
	if cursor := mustCursor(t, rtB.store, webhookB.ID, "r-b"); cursor == seqB {
		t.Fatal("disabled tenant B webhook must not advance its cursor")
	}

	storedB.Enabled = true
	if _, err := rtB.store.updateWebhook(storedB); err != nil {
		t.Fatal(err)
	}
	manager.processAllWebhooks(context.Background())

	if got := deliveries.Load(); got != 2 {
		t.Fatalf("deliveries = %d, want both tenants (2)", got)
	}
	if cursor := mustCursor(t, rtB.store, webhookB.ID, "r-b"); cursor != seqB {
		t.Fatalf("tenant B cursor = %d, want %d", cursor, seqB)
	}
}

// TestStartEventWorkerDeliversQueuedEvent exercises the public worker startup
// path, including the initial drain before the first ticker tick.
func TestStartEventWorkerDeliversQueuedEvent(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r", RuleMatch{TextContains: []string{"urgente"}})

	delivered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case delivered <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "r")
	seedEvent(t, rt.store, "m1", "5511@s.whatsapp.net", "urgente", false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.StartEventWorker(ctx)

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("event worker did not drain the queued event")
	}
}

// TestProcessAllWebhooksStopsWhenContextIsDone makes sure shutdown does not
// start another delivery after cancellation.
func TestProcessAllWebhooksStopsWhenContextIsDone(t *testing.T) {
	t.Setenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE", "1")
	manager, token := newTestManager(t)
	rt := attachTestRuntime(t, manager, LegacyTenantID, TenantDisconnected)
	handler := newTenantRESTHandler(manager)

	createRuleViaAPI(t, handler, token, LegacyTenantID, "r", RuleMatch{TextContains: []string{"urgente"}})

	var deliveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deliveries.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	createWebhookViaAPI(t, handler, token, LegacyTenantID, server.URL, "r")
	seedEvent(t, rt.store, "m1", "5511@s.whatsapp.net", "urgente", false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager.processAllWebhooks(ctx)

	if got := deliveries.Load(); got != 0 {
		t.Fatalf("deliveries = %d, want 0 after cancellation", got)
	}
}

func mustCursor(t *testing.T, store *MessageStore, webhookID, ruleID string) int64 {
	t.Helper()
	cursor, err := store.consumerCursor(webhookConsumerID(webhookID), ruleID)
	if err != nil {
		t.Fatal(err)
	}
	return cursor
}
