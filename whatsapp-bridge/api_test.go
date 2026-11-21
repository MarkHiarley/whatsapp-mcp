package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	waLog "go.mau.fi/whatsmeow/util/log"
)

func newTestManager(t *testing.T) (*TenantManager, string) {
	t.Helper()
	return newTestManagerWithAdmin(t, "")
}

func newTestManagerWithAdmin(t *testing.T, adminToken string) (*TenantManager, string) {
	t.Helper()
	t.Setenv("WHATSAPP_ADMIN_TOKEN", adminToken)
	store, err := NewTenantStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const legacyToken = "legacy-test-token"
	if err := store.EnsureDefault(legacyToken); err != nil {
		t.Fatal(err)
	}
	manager := NewTenantManager(store, waLog.Noop)
	t.Cleanup(manager.StopAll)
	return manager, legacyToken
}

func attachTestRuntime(t *testing.T, manager *TenantManager, tenantID string, status TenantStatus) *tenantRuntime {
	t.Helper()
	dir := t.TempDir()
	store, err := NewMessageStoreAt(dir+"/messages.db", dir+"/media")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rt := &tenantRuntime{
		tenant: Tenant{ID: tenantID, Label: tenantID},
		store:  store,
		status: status,
		qrSubs: make(map[chan qrEvent]struct{}),
		logger: waLog.Noop,
	}
	manager.mu.Lock()
	manager.runtimes[tenantID] = rt
	manager.mu.Unlock()
	return rt
}

func doRequest(t *testing.T, handler http.Handler, method, path, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func decodeEnvelope(t *testing.T, response *httptest.ResponseRecorder) APIResponse {
	t.Helper()
	var payload APIResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("invalid JSON envelope: %v (%q)", err, response.Body.String())
	}
	return payload
}

func TestV1ErrorContract(t *testing.T) {
	manager, token := newTestManager(t)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodPost, "/api/v1/send", token, `{"recipient":""}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type %q", response.Header().Get("Content-Type"))
	}
	payload := decodeEnvelope(t, response)
	if payload.Success || payload.Data != nil || payload.Error == nil || payload.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("unexpected contract response: %+v", payload)
	}
}

func TestV1RejectsUnknownFields(t *testing.T) {
	manager, token := newTestManager(t)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodPost, "/api/v1/send", token,
		`{"recipient":"123","message":"test","unknown":true}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestV1SuccessContract(t *testing.T) {
	response := httptest.NewRecorder()
	writeJSON(response, http.StatusOK, APIResponse{
		Success: true,
		Data:    SendMessageData{Message: "sent"},
		Error:   nil,
	})

	var payload struct {
		Success bool            `json:"success"`
		Data    SendMessageData `json:"data"`
		Error   *APIError       `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success || payload.Data.Message != "sent" || payload.Error != nil {
		t.Fatalf("unexpected contract response: %+v", payload)
	}
}

func TestLegacyRouteKeepsItsShape(t *testing.T) {
	manager, token := newTestManager(t)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodPost, "/api/send", token, `{"recipient":"5511999999999","message":"hi"}`)

	if response.Header().Get("Deprecation") != "true" {
		t.Fatal("legacy route must include the Deprecation header")
	}
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusInternalServerError)
	}
	var payload SendMessageResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("legacy route must answer {success,message}: %v", err)
	}
	if payload.Success || payload.Message == "" {
		t.Fatalf("unexpected legacy response: %+v", payload)
	}
}

func TestLegacyRouteValidationMessage(t *testing.T) {
	manager, token := newTestManager(t)
	handler := newTenantRESTHandler(manager)

	response := doRequest(t, handler, http.MethodPost, "/api/send", token, `{"recipient":""}`)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "Recipient is required") {
		t.Fatalf("unexpected legacy response: %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Deprecation") != "true" {
		t.Fatal("legacy route must include the Deprecation header")
	}
}
