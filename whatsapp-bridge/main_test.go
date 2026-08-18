package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireBearerToken(t *testing.T) {
	handler := requireBearerToken("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for name, test := range map[string]struct {
		authorization string
		want          int
	}{
		"missing": {"", http.StatusUnauthorized},
		"invalid": {"Bearer wrong", http.StatusUnauthorized},
		"valid":   {"Bearer secret", http.StatusNoContent},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/send", nil)
			req.Header.Set("Authorization", test.authorization)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != test.want {
				t.Fatalf("got status %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestV1ErrorContract(t *testing.T) {
	handler := newRESTHandler(nil, nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(`{"recipient":""}`))
	req.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type %q", response.Header().Get("Content-Type"))
	}
	var payload APIResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Success || payload.Data != nil || payload.Error == nil || payload.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("unexpected contract response: %+v", payload)
	}
}

func TestV1RejectsUnknownFields(t *testing.T) {
	handler := newRESTHandler(nil, nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(`{"recipient":"123","message":"test","unknown":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

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

func TestLegacyRouteRemainsAvailable(t *testing.T) {
	handler := newRESTHandler(nil, nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(`{"recipient":""}`))
	req.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "Recipient is required") {
		t.Fatalf("unexpected legacy response: %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Deprecation") != "true" {
		t.Fatal("legacy route must include the Deprecation header")
	}
}
