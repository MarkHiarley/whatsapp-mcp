package main

import (
	"net/http"
	"net/http/httptest"
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
