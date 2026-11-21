package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"time"
)

// LegacyTenantID is the tenant that keeps the pre-multi-tenant paths working.
const LegacyTenantID = "default"

// TenantKey is one credential of a tenant. The raw token is never persisted.
type TenantKey struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Hash       string     `json:"hash"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// Tenant is one WhatsApp account managed by the bridge.
type Tenant struct {
	ID        string      `json:"id"`
	Label     string      `json:"label"`
	CreatedAt time.Time   `json:"created_at"`
	Keys      []TenantKey `json:"keys"`
}

// tenantFile is the on-disk shape of store/tenants.json.
type tenantFile struct {
	Tenants []Tenant `json:"tenants"`
}

// TenantStatus is the connection state exposed by the API.
type TenantStatus string

const (
	TenantDisconnected TenantStatus = "disconnected"
	TenantPairing      TenantStatus = "pairing"
	TenantConnected    TenantStatus = "connected"
	TenantLoggedOut    TenantStatus = "logged_out"
)

// TenantView is the public projection; it never carries key hashes.
type TenantView struct {
	ID        string       `json:"id"`
	Label     string       `json:"label"`
	Status    TenantStatus `json:"status"`
	JID       string       `json:"jid,omitempty"`
	Keys      int          `json:"keys"`
	CreatedAt time.Time    `json:"created_at"`
}

var tenantIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,31}$`)

// ValidTenantID accepts the only tenant IDs allowed in URLs and paths.
func ValidTenantID(id string) bool {
	return tenantIDPattern.MatchString(id)
}

// HashTenantToken returns the hex SHA-256 stored in tenants.json.
func HashTenantToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewTenantToken creates a 256-bit bearer token.
func NewTenantToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// NewTenantKeyID creates the short identifier used to revoke one key.
func NewTenantKeyID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "key_" + hex.EncodeToString(buf), nil
}
