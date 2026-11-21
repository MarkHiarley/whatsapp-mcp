package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type createTenantRequest struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type createTenantKeyRequest struct {
	Label string `json:"label"`
}

// tenantKeyView is the public projection of one key (never the hash).
type tenantKeyView struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type createTenantData struct {
	Tenant TenantView    `json:"tenant"`
	Key    tenantKeyView `json:"key"`
	Token  string        `json:"token"`
}

type tenantKeyData struct {
	Key   tenantKeyView `json:"key"`
	Token string        `json:"token"`
}

type tenantAPI struct {
	m *TenantManager
}

func newTenantRESTHandler(m *TenantManager) http.Handler {
	api := &tenantAPI{m: m}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/tenants", api.authenticate(api.handleTenants))
	mux.HandleFunc("/api/v1/tenants/{tenant}", api.authenticate(api.handleTenant))
	mux.HandleFunc("/api/v1/tenants/{tenant}/status", api.authenticate(api.handleTenantStatus))
	mux.HandleFunc("/api/v1/tenants/{tenant}/qr", api.authenticate(api.handleTenantQR))
	mux.HandleFunc("/api/v1/tenants/{tenant}/keys", api.authenticate(api.handleTenantKeys))
	mux.HandleFunc("/api/v1/tenants/{tenant}/keys/{keyId}", api.authenticate(api.handleTenantKey))
	mux.HandleFunc("/api/v1/tenants/{tenant}/send", api.authenticate(api.handleTenantSend))
	mux.HandleFunc("/api/v1/tenants/{tenant}/download", api.authenticate(api.handleTenantDownload))
	mux.HandleFunc("/api/v1/tenants/{tenant}/events", api.authenticate(api.handleTenantEvents))
	mux.HandleFunc("/api/v1/tenants/{tenant}/consumers/{consumerId}/ack", api.authenticate(api.handleTenantConsumerAck))
	mux.HandleFunc("/api/v1/tenants/{tenant}/rules", api.authenticate(api.handleTenantRules))
	mux.HandleFunc("/api/v1/tenants/{tenant}/rules/{ruleId}", api.authenticate(api.handleTenantRule))
	mux.HandleFunc("/api/v1/tenants/{tenant}/webhooks", api.authenticate(api.handleTenantWebhooks))
	mux.HandleFunc("/api/v1/tenants/{tenant}/webhooks/{webhookId}", api.authenticate(api.handleTenantWebhook))
	mux.HandleFunc("/api/v1/tenants/{tenant}/webhooks/{webhookId}/secret", api.authenticate(api.handleTenantWebhookSecret))
	mux.HandleFunc("/api/v1/tenants/{tenant}/webhooks/{webhookId}/deliveries", api.authenticate(api.handleTenantWebhookDeliveries))

	mux.HandleFunc("/api/v1/send", api.authenticate(api.handleDefaultSend(true, false)))
	mux.HandleFunc("/api/v1/download", api.authenticate(api.handleDefaultDownload(true, false)))
	mux.HandleFunc("/api/send", api.authenticate(api.handleDefaultSend(false, true)))
	mux.HandleFunc("/api/download", api.authenticate(api.handleDefaultDownload(false, true)))

	return mux
}

// startRESTServer exposes the tenant API on port.
func startRESTServer(m *TenantManager, port int) {
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting REST API server on %s...\n", serverAddr)
	go func() {
		if err := http.ListenAndServe(serverAddr, newTenantRESTHandler(m)); err != nil {
			fmt.Printf("REST API server error: %v\n", err)
		}
	}()
}

// authenticate resolves the bearer token into a tenant or admin principal.
func (a *tenantAPI) authenticate(next func(http.ResponseWriter, *http.Request, principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing bearer token")
			return
		}
		p, ok := a.m.tenants.Authenticate(token)
		if !ok && a.m.AuthenticateAdmin(token) {
			p = principal{IsAdmin: true}
			ok = true
		}
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or revoked bearer token")
			return
		}
		next(w, r, p)
	}
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// authorize validates the path tenant and the caller's ownership of it.
func (a *tenantAPI) authorize(w http.ResponseWriter, p principal, tenant string) (string, bool) {
	if !ValidTenantID(tenant) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid tenant id")
		return "", false
	}
	if !p.IsAdmin && p.TenantID != tenant {
		writeAPIError(w, http.StatusForbidden, "FORBIDDEN", "Key does not belong to this tenant")
		return "", false
	}
	return tenant, true
}

func (a *tenantAPI) requireAdmin(w http.ResponseWriter, p principal) bool {
	if !a.m.AdminEnabled() {
		writeAPIError(w, http.StatusServiceUnavailable, "ADMIN_DISABLED", "Admin API is disabled")
		return false
	}
	if !p.IsAdmin {
		writeAPIError(w, http.StatusForbidden, "FORBIDDEN", "Admin token required")
		return false
	}
	return true
}

func methodNotAllowed(w http.ResponseWriter) {
	writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
}

func writeEnvelope(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, APIResponse{Success: true, Data: data, Error: nil})
}

func decodeStrict(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func tenantKeyViewOf(key TenantKey) tenantKeyView {
	return tenantKeyView{
		ID:         key.ID,
		Label:      key.Label,
		CreatedAt:  key.CreatedAt,
		LastUsedAt: key.LastUsedAt,
		RevokedAt:  key.RevokedAt,
	}
}

func tenantKeyViews(tenant Tenant) []tenantKeyView {
	views := make([]tenantKeyView, 0, len(tenant.Keys))
	for _, key := range tenant.Keys {
		views = append(views, tenantKeyViewOf(key))
	}
	return views
}

func (a *tenantAPI) handleTenants(w http.ResponseWriter, r *http.Request, p principal) {
	if !a.requireAdmin(w, p) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeEnvelope(w, http.StatusOK, struct {
			Tenants []TenantView `json:"tenants"`
		}{Tenants: a.m.Views()})
	case http.MethodPost:
		var req createTenantRequest
		if err := decodeStrict(r, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
			return
		}
		if !ValidTenantID(req.ID) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid tenant id")
			return
		}
		if strings.TrimSpace(req.Label) == "" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Label is required")
			return
		}
		tenant, keyID, token, err := a.m.CreateTenant(req.ID, req.Label)
		if err != nil {
			switch {
			case errors.Is(err, errTenantExists):
				writeAPIError(w, http.StatusConflict, "CONFLICT", "Tenant already exists")
			case errors.Is(err, errInvalidTenantID):
				writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid tenant id")
			default:
				writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			}
			return
		}
		var key tenantKeyView
		for _, candidate := range tenant.Keys {
			if candidate.ID == keyID {
				key = tenantKeyViewOf(candidate)
				break
			}
		}
		writeEnvelope(w, http.StatusCreated, createTenantData{
			Tenant: tenantView(tenant, TenantDisconnected, ""),
			Key:    key,
			Token:  token,
		})
	default:
		methodNotAllowed(w)
	}
}

func (a *tenantAPI) handleTenant(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	if !a.requireAdmin(w, p) {
		return
	}
	tenant, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if err := a.m.DeleteTenant(tenant); err != nil {
		if errors.Is(err, errTenantNotFound) {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		DeletedID string `json:"deleted_id"`
	}{DeletedID: tenant})
}

func (a *tenantAPI) tenantView(id string) (TenantView, bool) {
	if rt, ok := a.m.Runtime(id); ok {
		return rt.View(), true
	}
	if tenant, ok := a.m.tenants.Get(id); ok {
		return tenantView(tenant, TenantDisconnected, ""), true
	}
	return TenantView{}, false
}

func (a *tenantAPI) handleTenantStatus(w http.ResponseWriter, r *http.Request, p principal) {
	tenant, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	view, ok := a.tenantView(tenant)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		Tenant TenantView `json:"tenant"`
	}{Tenant: view})
}

func (a *tenantAPI) handleTenantQR(w http.ResponseWriter, r *http.Request, p principal) {
	tenant, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rt, ok := a.m.Runtime(tenant)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	events, err := a.m.StartPairing(rt)
	if err != nil {
		writeSSEStatus(w, flusher, rt.Status())
		return
	}
	defer a.m.UnsubscribeQR(rt, events)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case ev, open := <-events:
			if !open {
				return
			}
			switch ev.Kind {
			case "qr":
				fmt.Fprintf(w, "event: qr\ndata: {\"code\":%q}\n\n", ev.Code)
			case "status":
				fmt.Fprintf(w, "event: status\ndata: {\"status\":%q}\n\n", ev.Status)
			}
			flusher.Flush()
			if ev.Kind == "status" && (ev.Status == TenantConnected || ev.Status == TenantLoggedOut) {
				return
			}
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeSSEStatus(w http.ResponseWriter, flusher http.Flusher, status TenantStatus) {
	fmt.Fprintf(w, "event: status\ndata: {\"status\":%q}\n\n", status)
	flusher.Flush()
}

func (a *tenantAPI) handleTenantKeys(w http.ResponseWriter, r *http.Request, p principal) {
	tenantID, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	tenant, exists := a.m.tenants.Get(tenantID)
	if !exists {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeEnvelope(w, http.StatusOK, struct {
			Keys []tenantKeyView `json:"keys"`
		}{Keys: tenantKeyViews(tenant)})
	case http.MethodPost:
		var req createTenantKeyRequest
		if err := decodeStrict(r, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
			return
		}
		label := strings.TrimSpace(req.Label)
		if label == "" || len(label) > 64 {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Label must have 1 to 64 characters")
			return
		}
		key, token, err := a.m.tenants.CreateKey(tenantID, label)
		if err != nil {
			if errors.Is(err, errTenantNotFound) {
				writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		writeEnvelope(w, http.StatusCreated, tenantKeyData{Key: tenantKeyViewOf(key), Token: token})
	default:
		methodNotAllowed(w)
	}
}

func (a *tenantAPI) handleTenantKey(w http.ResponseWriter, r *http.Request, p principal) {
	tenantID, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	keyID := r.PathValue("keyId")
	if keyID == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Key id is required")
		return
	}
	if err := a.m.tenants.RevokeKey(tenantID, keyID); err != nil {
		if errors.Is(err, errTenantNotFound) || errors.Is(err, errKeyNotFound) {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Key not found")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		RevokedID string `json:"revoked_id"`
	}{RevokedID: keyID})
}

// decodeSend validates a send request body.
func decodeSend(r *http.Request, legacy bool) (SendMessageRequest, error) {
	var req SendMessageRequest
	var err error
	if legacy {
		err = json.NewDecoder(r.Body).Decode(&req)
	} else {
		err = decodeStrict(r, &req)
	}
	if err != nil {
		return req, fmt.Errorf("Invalid request format")
	}
	if req.Recipient == "" {
		return req, fmt.Errorf("Recipient is required")
	}
	if req.Message == "" && req.MediaPath == "" {
		return req, fmt.Errorf("Message or media path is required")
	}
	return req, nil
}

func decodeDownload(r *http.Request, legacy bool) (DownloadMediaRequest, error) {
	var req DownloadMediaRequest
	var err error
	if legacy {
		err = json.NewDecoder(r.Body).Decode(&req)
	} else {
		err = decodeStrict(r, &req)
	}
	if err != nil {
		return req, fmt.Errorf("Invalid request format")
	}
	if req.MessageID == "" || req.ChatJID == "" {
		return req, fmt.Errorf("Message ID and chat JID are required")
	}
	return req, nil
}

func (a *tenantAPI) handleTenantSend(w http.ResponseWriter, r *http.Request, p principal) {
	tenantID, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	req, err := decodeSend(r, false)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rt, ok := a.m.Runtime(tenantID)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return
	}
	if rt.Status() != TenantConnected || rt.client == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "TENANT_DISCONNECTED", "Tenant is not connected")
		return
	}
	success, message := sendWhatsAppMessage(rt.client, req.Recipient, req.Message, req.MediaPath)
	if !success {
		writeAPIError(w, http.StatusInternalServerError, "SEND_FAILED", message)
		return
	}
	writeEnvelope(w, http.StatusOK, SendMessageData{Message: message})
}

func (a *tenantAPI) handleTenantDownload(w http.ResponseWriter, r *http.Request, p principal) {
	tenantID, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	req, err := decodeDownload(r, false)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	rt, ok := a.m.Runtime(tenantID)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return
	}
	if rt.Status() != TenantConnected || rt.client == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "TENANT_DISCONNECTED", "Tenant is not connected")
		return
	}
	success, mediaType, filename, path, err := downloadMedia(rt.client, rt.store, rt.store.MediaDir(), req.MessageID, req.ChatJID)
	if !success || err != nil {
		message := "Failed to download media"
		if err != nil {
			message += ": " + err.Error()
		}
		writeAPIError(w, http.StatusInternalServerError, "DOWNLOAD_FAILED", message)
		return
	}
	writeEnvelope(w, http.StatusOK, DownloadMediaData{
		Message:  fmt.Sprintf("Successfully downloaded %s media", mediaType),
		Filename: filename,
		Path:     path,
	})
}

// handleDefaultSend serves the default tenant through the legacy paths.
func (a *tenantAPI) handleDefaultSend(versioned, legacy bool) func(http.ResponseWriter, *http.Request, principal) {
	return func(w http.ResponseWriter, r *http.Request, p principal) {
		if legacy {
			w.Header().Set("Deprecation", "true")
		}
		if !p.IsAdmin && p.TenantID != LegacyTenantID {
			contractError(w, http.StatusForbidden, "FORBIDDEN", "Key does not belong to this tenant", legacy)
			return
		}
		if r.Method != http.MethodPost {
			contractError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", legacy)
			return
		}
		req, err := decodeSend(r, legacy)
		if err != nil {
			contractError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), legacy)
			return
		}

		rt, ok := a.m.Runtime(LegacyTenantID)
		if versioned && !ok {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
			return
		}
		if versioned && (rt.Status() != TenantConnected || rt.client == nil) {
			writeAPIError(w, http.StatusServiceUnavailable, "TENANT_DISCONNECTED", "Tenant is not connected")
			return
		}

		success := false
		message := "Tenant is not connected"
		if ok && rt.client != nil {
			success, message = sendWhatsAppMessage(rt.client, req.Recipient, req.Message, req.MediaPath)
		}

		if legacy {
			status := http.StatusOK
			if !success {
				status = http.StatusInternalServerError
			}
			writeJSON(w, status, SendMessageResponse{Success: success, Message: message})
			return
		}
		if !success {
			writeAPIError(w, http.StatusInternalServerError, "SEND_FAILED", message)
			return
		}
		writeEnvelope(w, http.StatusOK, SendMessageData{Message: message})
	}
}

// handleDefaultDownload serves the default tenant through the legacy paths.
func (a *tenantAPI) handleDefaultDownload(versioned, legacy bool) func(http.ResponseWriter, *http.Request, principal) {
	return func(w http.ResponseWriter, r *http.Request, p principal) {
		if legacy {
			w.Header().Set("Deprecation", "true")
		}
		if !p.IsAdmin && p.TenantID != LegacyTenantID {
			contractError(w, http.StatusForbidden, "FORBIDDEN", "Key does not belong to this tenant", legacy)
			return
		}
		if r.Method != http.MethodPost {
			contractError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", legacy)
			return
		}
		req, err := decodeDownload(r, legacy)
		if err != nil {
			contractError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), legacy)
			return
		}

		rt, ok := a.m.Runtime(LegacyTenantID)
		if versioned && !ok {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
			return
		}
		if versioned && (rt.Status() != TenantConnected || rt.client == nil) {
			writeAPIError(w, http.StatusServiceUnavailable, "TENANT_DISCONNECTED", "Tenant is not connected")
			return
		}

		var success bool
		var mediaType, filename, path string
		var downloadErr error
		if ok && rt.client != nil && rt.store != nil {
			success, mediaType, filename, path, downloadErr = downloadMedia(rt.client, rt.store, rt.store.MediaDir(), req.MessageID, req.ChatJID)
		} else {
			downloadErr = fmt.Errorf("tenant is not connected")
		}

		if legacy {
			if !success || downloadErr != nil {
				message := "Failed to download media"
				if downloadErr != nil {
					message += ": " + downloadErr.Error()
				}
				writeJSON(w, http.StatusInternalServerError, DownloadMediaResponse{Success: false, Message: message})
				return
			}
			writeJSON(w, http.StatusOK, DownloadMediaResponse{
				Success:  true,
				Message:  fmt.Sprintf("Successfully downloaded %s media", mediaType),
				Filename: filename,
				Path:     path,
			})
			return
		}
		if !success || downloadErr != nil {
			message := "Failed to download media"
			if downloadErr != nil {
				message += ": " + downloadErr.Error()
			}
			writeAPIError(w, http.StatusInternalServerError, "DOWNLOAD_FAILED", message)
			return
		}
		writeEnvelope(w, http.StatusOK, DownloadMediaData{
			Message:  fmt.Sprintf("Successfully downloaded %s media", mediaType),
			Filename: filename,
			Path:     path,
		})
	}
}
