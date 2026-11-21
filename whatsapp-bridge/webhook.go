package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// EventWebhook is one subscribed delivery target of a tenant.
type EventWebhook struct {
	ID        string
	URL       string
	Secret    string
	RuleID    string
	Enabled   bool
	CreatedAt time.Time
}

// webhookView is the public projection; it never carries the secret.
type webhookView struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	RuleID    string    `json:"rule_id"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

func webhookViewOf(wh EventWebhook) webhookView {
	return webhookView{
		ID:        wh.ID,
		URL:       wh.URL,
		RuleID:    wh.RuleID,
		Enabled:   wh.Enabled,
		CreatedAt: wh.CreatedAt,
	}
}

// DeliveryView is one row of the webhook delivery log.
type DeliveryView struct {
	Seq           int64      `json:"seq"`
	WebhookID     string     `json:"webhook_id"`
	Attempts      int        `json:"attempts"`
	LastStatus    *int       `json:"last_status,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
}

const webhookDeliveryAttemptsLimit = 8

// webhookHTTPClient is shared by every delivery; redirects are never followed.
var webhookHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func newWebhookSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// validateWebhookURL blocks SSRF targets: only http(s) hosts resolving to
// public addresses are accepted unless WHATSAPP_WEBHOOK_ALLOW_PRIVATE=1.
func validateWebhookURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("webhook url must use http or https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return fmt.Errorf("webhook url must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("webhook url must not include userinfo")
	}

	ips, err := net.LookupIP(parsed.Hostname())
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("webhook host cannot be resolved")
	}
	allowPrivate := os.Getenv("WHATSAPP_WEBHOOK_ALLOW_PRIVATE") == "1"
	for _, ip := range ips {
		if !isRestrictedIP(ip) {
			continue
		}
		if allowPrivate {
			continue
		}
		return fmt.Errorf("webhook url points to a restricted address")
	}
	return nil
}

func isRestrictedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// listWebhooks returns every webhook of the tenant in creation order.
func (store *MessageStore) listWebhooks() ([]EventWebhook, error) {
	rows, err := store.db.Query(`
		SELECT id, url, secret, rule_id, enabled, created_at
		FROM event_webhooks ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	webhooks := make([]EventWebhook, 0)
	for rows.Next() {
		wh, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, rows.Err()
}

// getWebhook loads one webhook; the boolean reports existence.
func (store *MessageStore) getWebhook(id string) (EventWebhook, bool, error) {
	row := store.db.QueryRow(`
		SELECT id, url, secret, rule_id, enabled, created_at
		FROM event_webhooks WHERE id = ?
	`, id)
	wh, err := scanWebhook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return EventWebhook{}, false, nil
	}
	if err != nil {
		return EventWebhook{}, false, err
	}
	return wh, true, nil
}

func scanWebhook(row rowScanner) (EventWebhook, error) {
	var wh EventWebhook
	err := row.Scan(&wh.ID, &wh.URL, &wh.Secret, &wh.RuleID, &wh.Enabled, &wh.CreatedAt)
	return wh, err
}

func (store *MessageStore) insertWebhook(wh EventWebhook) error {
	_, err := store.db.Exec(`
		INSERT INTO event_webhooks (id, url, secret, rule_id, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, wh.ID, wh.URL, wh.Secret, wh.RuleID, wh.Enabled, wh.CreatedAt)
	return err
}

func (store *MessageStore) updateWebhook(wh EventWebhook) (bool, error) {
	result, err := store.db.Exec(`
		UPDATE event_webhooks SET url = ?, rule_id = ?, enabled = ? WHERE id = ?
	`, wh.URL, wh.RuleID, wh.Enabled, wh.ID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (store *MessageStore) deleteWebhook(id string) (bool, error) {
	result, err := store.db.Exec("DELETE FROM event_webhooks WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (store *MessageStore) rotateWebhookSecret(id, secret string) (bool, error) {
	result, err := store.db.Exec("UPDATE event_webhooks SET secret = ? WHERE id = ?", secret, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// listDeliveries returns the newest delivery rows first.
func (store *MessageStore) listDeliveries(webhookID string, limit int) ([]DeliveryView, error) {
	rows, err := store.db.Query(`
		SELECT seq, webhook_id, attempts, last_status, last_error, next_attempt_at, delivered_at
		FROM event_deliveries WHERE webhook_id = ?
		ORDER BY seq DESC LIMIT ?
	`, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := make([]DeliveryView, 0, limit)
	for rows.Next() {
		var (
			d             DeliveryView
			lastStatus    sql.NullInt64
			lastError     sql.NullString
			nextAttemptAt sql.NullTime
			deliveredAt   sql.NullTime
		)
		if err := rows.Scan(&d.Seq, &d.WebhookID, &d.Attempts, &lastStatus, &lastError, &nextAttemptAt, &deliveredAt); err != nil {
			return nil, err
		}
		if lastStatus.Valid {
			status := int(lastStatus.Int64)
			d.LastStatus = &status
		}
		d.LastError = lastError.String
		if nextAttemptAt.Valid {
			value := nextAttemptAt.Time
			d.NextAttemptAt = &value
		}
		if deliveredAt.Valid {
			value := deliveredAt.Time
			d.DeliveredAt = &value
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// deliveryState is the mutable part of one delivery row.
type deliveryState struct {
	Attempts      int
	LastStatus    *int
	LastError     string
	NextAttemptAt *time.Time
	DeliveredAt   *time.Time
}

func (store *MessageStore) deliveryFor(seq int64, webhookID string) (deliveryState, bool, error) {
	var (
		state         deliveryState
		lastStatus    sql.NullInt64
		lastError     sql.NullString
		nextAttemptAt sql.NullTime
		deliveredAt   sql.NullTime
	)
	err := store.db.QueryRow(`
		SELECT attempts, last_status, last_error, next_attempt_at, delivered_at
		FROM event_deliveries WHERE seq = ? AND webhook_id = ?
	`, seq, webhookID).Scan(&state.Attempts, &lastStatus, &lastError, &nextAttemptAt, &deliveredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return deliveryState{}, false, nil
	}
	if err != nil {
		return deliveryState{}, false, err
	}
	if lastStatus.Valid {
		status := int(lastStatus.Int64)
		state.LastStatus = &status
	}
	state.LastError = lastError.String
	if nextAttemptAt.Valid {
		value := nextAttemptAt.Time
		state.NextAttemptAt = &value
	}
	if deliveredAt.Valid {
		value := deliveredAt.Time
		state.DeliveredAt = &value
	}
	return state, true, nil
}

func (store *MessageStore) createDelivery(seq int64, webhookID string) error {
	_, err := store.db.Exec(`
		INSERT INTO event_deliveries (seq, webhook_id, attempts, last_status, last_error, next_attempt_at, delivered_at)
		VALUES (?, ?, 0, NULL, NULL, NULL, NULL)
		ON CONFLICT(seq, webhook_id) DO NOTHING
	`, seq, webhookID)
	return err
}

func (store *MessageStore) saveDelivery(seq int64, webhookID string, state deliveryState) error {
	_, err := store.db.Exec(`
		UPDATE event_deliveries SET
			attempts = ?, last_status = ?, last_error = ?, next_attempt_at = ?, delivered_at = ?
		WHERE seq = ? AND webhook_id = ?
	`, state.Attempts, nullableInt(state.LastStatus), nullableString(state.LastError),
		nullableTime(state.NextAttemptAt), nullableTime(state.DeliveredAt), seq, webhookID)
	return err
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

// webhookConsumerID namespaces the cursor of one webhook consumer.
func webhookConsumerID(webhookID string) string {
	return "webhook:" + webhookID
}

// eventSignature computes the X-WhatsApp-Signature value for one body.
func eventSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// webhookBackoff returns 2^attempts seconds capped at five minutes.
func webhookBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 20 {
		return 5 * time.Minute
	}
	delay := time.Duration(1<<uint(attempts)) * time.Second
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return delay
}

// deliverEvent posts one event body; status is nil for transport failures.
func deliverEvent(ctx context.Context, wh EventWebhook, tenantID string, seq int64, body []byte) (*int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-WhatsApp-Event-Id", tenantID+":"+strconv.FormatInt(seq, 10))
	req.Header.Set("X-WhatsApp-Signature", eventSignature(wh.Secret, body))

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	status := resp.StatusCode
	return &status, nil
}

// processWebhookOnce advances one webhook by at most one event. It returns
// true when the cursor or the delivery log changed.
func (m *TenantManager) processWebhookOnce(ctx context.Context, rt *tenantRuntime, webhookID string) (bool, error) {
	wh, ok, err := rt.store.getWebhook(webhookID)
	if err != nil {
		return false, err
	}
	if !ok || !wh.Enabled {
		return false, nil
	}
	rule, ok, err := rt.store.getRule(wh.RuleID)
	if err != nil {
		return false, err
	}
	if !ok {
		// The rule was deleted; the webhook is inert until it is repointed.
		return false, nil
	}

	consumerID := webhookConsumerID(webhookID)
	cursor, err := rt.store.consumerCursor(consumerID, wh.RuleID)
	if err != nil {
		return false, err
	}
	ev, found, err := rt.store.nextEventAfter(cursor)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	state, exists, err := rt.store.deliveryFor(ev.Seq, webhookID)
	if err != nil {
		return false, err
	}
	if !exists {
		if !rule.Match.Matches(ev) {
			if _, err := rt.store.ackConsumer(consumerID, wh.RuleID, ev.Seq); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := rt.store.createDelivery(ev.Seq, webhookID); err != nil {
			return false, err
		}
		state = deliveryState{}
	}

	if state.DeliveredAt != nil {
		if _, err := rt.store.ackConsumer(consumerID, wh.RuleID, ev.Seq); err != nil {
			return false, err
		}
		return true, nil
	}

	now := time.Now().UTC()
	if state.NextAttemptAt != nil && state.NextAttemptAt.After(now) {
		return false, nil
	}

	body, err := json.Marshal(ev)
	if err != nil {
		return false, err
	}
	status, deliveryErr := deliverEvent(ctx, wh, rt.tenant.ID, ev.Seq, body)
	state.Attempts++

	if deliveryErr == nil && status != nil && *status >= 200 && *status < 300 {
		state.LastStatus = status
		state.LastError = ""
		state.NextAttemptAt = nil
		state.DeliveredAt = &now
		if err := rt.store.saveDelivery(ev.Seq, webhookID, state); err != nil {
			return false, err
		}
		if _, err := rt.store.ackConsumer(consumerID, wh.RuleID, ev.Seq); err != nil {
			return false, err
		}
		return true, nil
	}

	message := "delivery failed"
	if deliveryErr != nil {
		message = deliveryErr.Error()
	} else if status != nil {
		message = fmt.Sprintf("http status %d", *status)
		state.LastStatus = status
	}

	if state.Attempts >= webhookDeliveryAttemptsLimit {
		state.LastError = "dead: " + message
		state.NextAttemptAt = nil
		state.DeliveredAt = nil
		if err := rt.store.saveDelivery(ev.Seq, webhookID, state); err != nil {
			return false, err
		}
		if _, err := rt.store.ackConsumer(consumerID, wh.RuleID, ev.Seq); err != nil {
			return false, err
		}
		return true, nil
	}

	next := now.Add(webhookBackoff(state.Attempts))
	state.LastError = message
	state.NextAttemptAt = &next
	state.DeliveredAt = nil
	if err := rt.store.saveDelivery(ev.Seq, webhookID, state); err != nil {
		return false, err
	}
	return true, nil
}

// StartEventWorker starts the single delivery goroutine of the manager.
func (m *TenantManager) StartEventWorker(ctx context.Context) {
	m.eventOnce.Do(func() {
		workerCtx, cancel := context.WithCancel(ctx)
		m.mu.Lock()
		m.eventCancel = cancel
		m.mu.Unlock()
		go m.runEventWorker(workerCtx)
	})
}

func (m *TenantManager) runEventWorker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	m.processAllWebhooks(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stop:
			return
		case <-ticker.C:
			m.processAllWebhooks(ctx)
		}
	}
}

// processAllWebhooks drains every enabled webhook, up to a safe burst.
func (m *TenantManager) processAllWebhooks(ctx context.Context) {
	m.mu.RLock()
	runtimes := make([]*tenantRuntime, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		runtimes = append(runtimes, rt)
	}
	m.mu.RUnlock()

	for _, rt := range runtimes {
		if ctx.Err() != nil {
			return
		}
		if rt.store == nil {
			continue
		}
		webhooks, err := rt.store.listWebhooks()
		if err != nil {
			m.logger.Errorf("Tenant %s: failed to list webhooks: %v", rt.tenant.ID, err)
			continue
		}
		for _, wh := range webhooks {
			if !wh.Enabled {
				continue
			}
			for range 100 {
				if ctx.Err() != nil {
					return
				}
				progress, err := m.processWebhookOnce(ctx, rt, wh.ID)
				if err != nil {
					m.logger.Errorf("Tenant %s: webhook %s delivery error: %v", rt.tenant.ID, wh.ID, err)
					break
				}
				if !progress {
					break
				}
			}
		}
	}
}
