package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ackRequest struct {
	RuleID string `json:"rule_id"`
	Seq    int64  `json:"seq"`
}

type createRuleRequest struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Match   RuleMatch `json:"match"`
	Enabled *bool     `json:"enabled"`
}

type updateRuleRequest struct {
	Name    *string    `json:"name"`
	Match   *RuleMatch `json:"match"`
	Enabled *bool      `json:"enabled"`
}

type createWebhookRequest struct {
	URL     string `json:"url"`
	RuleID  string `json:"rule_id"`
	Enabled *bool  `json:"enabled"`
}

type updateWebhookRequest struct {
	URL     *string `json:"url"`
	RuleID  *string `json:"rule_id"`
	Enabled *bool   `json:"enabled"`
}

// eventsRuntime resolves the path tenant and its live store.
func (a *tenantAPI) eventsRuntime(w http.ResponseWriter, p principal, r *http.Request) (*tenantRuntime, bool) {
	tenantID, ok := a.authorize(w, p, r.PathValue("tenant"))
	if !ok {
		return nil, false
	}
	rt, ok := a.m.Runtime(tenantID)
	if !ok || rt.store == nil {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return nil, false
	}
	return rt, true
}

func parseBoundedInt(raw string, fallback, min, max int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}
	return value, nil
}

func parseAfter(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid after cursor")
	}
	return value, nil
}

func (a *tenantAPI) handleTenantEvents(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	query := r.URL.Query()
	after, err := parseAfter(query.Get("after"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	limit, err := parseBoundedInt(query.Get("limit"), 100, 1, 500)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid limit")
		return
	}
	ruleID := strings.TrimSpace(query.Get("rule"))
	consumer := strings.TrimSpace(query.Get("consumer"))
	if consumer != "" && !ValidEventID(consumer) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid consumer id")
		return
	}

	var rule *EventRule
	if ruleID != "" {
		found, exists, err := rt.store.getRule(ruleID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		if !exists {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
			return
		}
		rule = &found
	}

	if consumer != "" && after == 0 {
		cursor, err := rt.store.consumerCursor(consumer, ruleID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		after = cursor
	}

	events, nextAfter, err := collectEvents(rt.store, after, limit, rule)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		Events    []Event `json:"events"`
		NextAfter int64   `json:"next_after"`
	}{Events: events, NextAfter: nextAfter})
}

// collectEvents reads in batches so filtered rules still advance the cursor.
func collectEvents(store *MessageStore, after int64, limit int, rule *EventRule) ([]Event, int64, error) {
	if rule == nil {
		events, err := store.listEventsAfter(after, limit)
		if err != nil {
			return nil, after, err
		}
		next := after
		if len(events) > 0 {
			next = events[len(events)-1].Seq
		}
		return events, next, nil
	}

	const batchSize = 500
	events := make([]Event, 0, limit)
	cursor := after

loop:
	for len(events) < limit {
		batch, err := store.listEventsAfter(cursor, batchSize)
		if err != nil {
			return nil, cursor, err
		}
		if len(batch) == 0 {
			break
		}
		for _, ev := range batch {
			cursor = ev.Seq
			if rule.Match.Matches(ev) {
				events = append(events, ev)
				if len(events) == limit {
					break loop
				}
			}
		}
		if len(batch) < batchSize {
			break
		}
	}
	return events, cursor, nil
}

func (a *tenantAPI) handleTenantConsumerAck(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	consumer := r.PathValue("consumerId")
	if !ValidEventID(consumer) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid consumer id")
		return
	}
	var req ackRequest
	if err := decodeStrict(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}
	if req.Seq < 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "seq must not be negative")
		return
	}
	lastSeq, err := rt.store.ackConsumer(consumer, req.RuleID, req.Seq)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		ConsumerID string `json:"consumer_id"`
		RuleID     string `json:"rule_id"`
		LastSeq    int64  `json:"last_seq"`
	}{ConsumerID: consumer, RuleID: req.RuleID, LastSeq: lastSeq})
}

func (a *tenantAPI) handleTenantRules(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rules, err := rt.store.listRules()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		writeEnvelope(w, http.StatusOK, struct {
			Rules []EventRule `json:"rules"`
		}{Rules: rules})
	case http.MethodPost:
		a.createRule(w, r, rt)
	default:
		methodNotAllowed(w)
	}
}

func (a *tenantAPI) createRule(w http.ResponseWriter, r *http.Request, rt *tenantRuntime) {
	var req createRuleRequest
	if err := decodeStrict(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Name is required")
		return
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		generated, err := newEventID("rule_")
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		id = generated
	} else if !ValidEventID(id) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid rule id")
		return
	}
	if err := req.Match.Validate(); err != nil {
		writeAPIError(w, http.StatusBadRequest, "RULE_INVALID", err.Error())
		return
	}
	if _, exists, err := rt.store.getRule(id); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	} else if exists {
		writeAPIError(w, http.StatusConflict, "CONFLICT", "Rule already exists")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := EventRule{
		ID:        id,
		Name:      name,
		Match:     req.Match,
		Enabled:   enabled,
		CreatedAt: time.Now().UTC(),
	}
	if err := rt.store.insertRule(rule); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusCreated, struct {
		Rule EventRule `json:"rule"`
	}{Rule: rule})
}

func (a *tenantAPI) handleTenantRule(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	ruleID := r.PathValue("ruleId")
	rule, exists, err := rt.store.getRule(ruleID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if !exists {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var req updateRuleRequest
		if err := decodeStrict(r, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
			return
		}
		if req.Name == nil && req.Match == nil && req.Enabled == nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "At least one field is required")
			return
		}
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Name is required")
				return
			}
			rule.Name = name
		}
		if req.Match != nil {
			if err := req.Match.Validate(); err != nil {
				writeAPIError(w, http.StatusBadRequest, "RULE_INVALID", err.Error())
				return
			}
			rule.Match = *req.Match
		}
		if req.Enabled != nil {
			rule.Enabled = *req.Enabled
		}
		if _, err := rt.store.updateRule(rule); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		writeEnvelope(w, http.StatusOK, struct {
			Rule EventRule `json:"rule"`
		}{Rule: rule})
	case http.MethodDelete:
		if _, err := rt.store.deleteRule(ruleID); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		writeEnvelope(w, http.StatusOK, struct {
			DeletedID string `json:"deleted_id"`
		}{DeletedID: ruleID})
	default:
		methodNotAllowed(w)
	}
}

func (a *tenantAPI) handleTenantWebhooks(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		webhooks, err := rt.store.listWebhooks()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		views := make([]webhookView, 0, len(webhooks))
		for _, wh := range webhooks {
			views = append(views, webhookViewOf(wh))
		}
		writeEnvelope(w, http.StatusOK, struct {
			Webhooks []webhookView `json:"webhooks"`
		}{Webhooks: views})
	case http.MethodPost:
		a.createWebhook(w, r, rt)
	default:
		methodNotAllowed(w)
	}
}

func (a *tenantAPI) createWebhook(w http.ResponseWriter, r *http.Request, rt *tenantRuntime) {
	var req createWebhookRequest
	if err := decodeStrict(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}
	if err := validateWebhookURL(req.URL); err != nil {
		writeAPIError(w, http.StatusBadRequest, "WEBHOOK_INVALID", err.Error())
		return
	}
	if _, exists, err := rt.store.getRule(req.RuleID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	} else if !exists {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
		return
	}

	id, err := newEventID("webhook_")
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	secret, err := newWebhookSecret()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	webhook := EventWebhook{
		ID:        id,
		URL:       strings.TrimSpace(req.URL),
		Secret:    secret,
		RuleID:    req.RuleID,
		Enabled:   enabled,
		CreatedAt: time.Now().UTC(),
	}
	if err := rt.store.insertWebhook(webhook); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusCreated, struct {
		Webhook webhookView `json:"webhook"`
		Secret  string      `json:"secret"`
	}{Webhook: webhookViewOf(webhook), Secret: secret})
}

func (a *tenantAPI) handleTenantWebhook(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	webhookID := r.PathValue("webhookId")
	webhook, exists, err := rt.store.getWebhook(webhookID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if !exists {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Webhook not found")
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var req updateWebhookRequest
		if err := decodeStrict(r, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
			return
		}
		if req.URL == nil && req.RuleID == nil && req.Enabled == nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "At least one field is required")
			return
		}
		if req.URL != nil {
			if err := validateWebhookURL(*req.URL); err != nil {
				writeAPIError(w, http.StatusBadRequest, "WEBHOOK_INVALID", err.Error())
				return
			}
			webhook.URL = strings.TrimSpace(*req.URL)
		}
		if req.RuleID != nil {
			if _, exists, err := rt.store.getRule(*req.RuleID); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
				return
			} else if !exists {
				writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
				return
			}
			webhook.RuleID = *req.RuleID
		}
		if req.Enabled != nil {
			webhook.Enabled = *req.Enabled
		}
		if _, err := rt.store.updateWebhook(webhook); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		writeEnvelope(w, http.StatusOK, struct {
			Webhook webhookView `json:"webhook"`
		}{Webhook: webhookViewOf(webhook)})
	case http.MethodDelete:
		if _, err := rt.store.deleteWebhook(webhookID); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		writeEnvelope(w, http.StatusOK, struct {
			DeletedID string `json:"deleted_id"`
		}{DeletedID: webhookID})
	default:
		methodNotAllowed(w)
	}
}

func (a *tenantAPI) handleTenantWebhookSecret(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	webhookID := r.PathValue("webhookId")
	if _, exists, err := rt.store.getWebhook(webhookID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	} else if !exists {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Webhook not found")
		return
	}
	secret, err := newWebhookSecret()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if _, err := rt.store.rotateWebhookSecret(webhookID, secret); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		Secret string `json:"secret"`
	}{Secret: secret})
}

func (a *tenantAPI) handleTenantWebhookDeliveries(w http.ResponseWriter, r *http.Request, p principal) {
	rt, ok := a.eventsRuntime(w, p, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	webhookID := r.PathValue("webhookId")
	if _, exists, err := rt.store.getWebhook(webhookID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	} else if !exists {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "Webhook not found")
		return
	}
	limit, err := parseBoundedInt(r.URL.Query().Get("limit"), 50, 1, 200)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid limit")
		return
	}
	deliveries, err := rt.store.listDeliveries(webhookID, limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeEnvelope(w, http.StatusOK, struct {
		Deliveries []DeliveryView `json:"deliveries"`
	}{Deliveries: deliveries})
}
