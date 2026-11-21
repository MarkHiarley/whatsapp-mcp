package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	errInvalidTenantID = errors.New("invalid tenant id")
	errTenantExists    = errors.New("tenant already exists")
	errTenantNotFound  = errors.New("tenant not found")
	errKeyNotFound     = errors.New("tenant key not found")
)

// principal is the authenticated caller of the HTTP API.
type principal struct {
	IsAdmin  bool
	TenantID string
	KeyID    string
}

// TenantStore owns <store>/tenants.json and is safe for concurrent use.
type TenantStore struct {
	mu       sync.Mutex
	root     string
	path     string
	file     tenantFile
	lastSave map[string]time.Time
}

// NewTenantStore loads (or starts) the tenant registry under root.
func NewTenantStore(root string) (*TenantStore, error) {
	if root == "" {
		return nil, fmt.Errorf("tenant store root is required")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}
	store := &TenantStore{
		root:     root,
		path:     filepath.Join(root, "tenants.json"),
		lastSave: make(map[string]time.Time),
	}
	data, err := os.ReadFile(store.path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &store.file); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", store.path, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// An absent registry is an empty registry.
	default:
		return nil, fmt.Errorf("failed to read %s: %w", store.path, err)
	}
	return store, nil
}

// Root returns the directory that owns the registry and the legacy paths.
func (s *TenantStore) Root() string {
	return s.root
}

// EnsureDefault guarantees that the legacy tenant exists.
func (s *TenantStore) EnsureDefault(legacyToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tenant := s.findLocked(LegacyTenantID); tenant != nil {
		if legacyToken == "" {
			return nil
		}
		hash := HashTenantToken(legacyToken)
		if tenant.hasKeyHash(hash) {
			return nil
		}
		keyID := "key_legacy"
		if tenant.hasKeyID(keyID) {
			generated, err := NewTenantKeyID()
			if err != nil {
				return err
			}
			keyID = generated
		}
		tenant.Keys = append(tenant.Keys, TenantKey{
			ID:        keyID,
			Label:     "legacy",
			Hash:      hash,
			CreatedAt: time.Now().UTC(),
		})
		return s.saveLocked()
	}

	if legacyToken == "" {
		return fmt.Errorf("legacy tenant is missing and WHATSAPP_API_TOKEN is empty")
	}
	s.file.Tenants = append(s.file.Tenants, Tenant{
		ID:        LegacyTenantID,
		Label:     "legacy",
		CreatedAt: time.Now().UTC(),
		Keys: []TenantKey{{
			ID:        "key_legacy",
			Label:     "legacy",
			Hash:      HashTenantToken(legacyToken),
			CreatedAt: time.Now().UTC(),
		}},
	})
	return s.saveLocked()
}

// List returns every tenant projected for the API, ordered by ID.
func (s *TenantStore) List() []TenantView {
	s.mu.Lock()
	defer s.mu.Unlock()

	views := make([]TenantView, 0, len(s.file.Tenants))
	for i := range s.file.Tenants {
		views = append(views, tenantView(s.file.Tenants[i], TenantDisconnected, ""))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views
}

// Get returns a defensive copy of one tenant.
func (s *TenantStore) Get(id string) (Tenant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant := s.findLocked(id)
	if tenant == nil {
		return Tenant{}, false
	}
	return cloneTenant(*tenant), true
}

// Create registers a new tenant and issues its first key.
func (s *TenantStore) Create(id, label string) (Tenant, string, string, error) {
	if !ValidTenantID(id) {
		return Tenant{}, "", "", errInvalidTenantID
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.findLocked(id) != nil {
		return Tenant{}, "", "", errTenantExists
	}
	keyID, err := NewTenantKeyID()
	if err != nil {
		return Tenant{}, "", "", err
	}
	token, err := NewTenantToken()
	if err != nil {
		return Tenant{}, "", "", err
	}
	tenant := Tenant{
		ID:        id,
		Label:     label,
		CreatedAt: time.Now().UTC(),
		Keys: []TenantKey{{
			ID:        keyID,
			Label:     label,
			Hash:      HashTenantToken(token),
			CreatedAt: time.Now().UTC(),
		}},
	}
	s.file.Tenants = append(s.file.Tenants, tenant)
	if err := s.saveLocked(); err != nil {
		s.file.Tenants = s.file.Tenants[:len(s.file.Tenants)-1]
		return Tenant{}, "", "", err
	}
	return cloneTenant(tenant), keyID, token, nil
}

// Delete removes a tenant from the registry without touching its files.
func (s *TenantStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.file.Tenants {
		if s.file.Tenants[i].ID == id {
			s.file.Tenants = append(s.file.Tenants[:i], s.file.Tenants[i+1:]...)
			return s.saveLocked()
		}
	}
	return errTenantNotFound
}

// CreateKey appends a new key to an existing tenant.
func (s *TenantStore) CreateKey(tenantID, label string) (TenantKey, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant := s.findLocked(tenantID)
	if tenant == nil {
		return TenantKey{}, "", errTenantNotFound
	}
	keyID, err := NewTenantKeyID()
	if err != nil {
		return TenantKey{}, "", err
	}
	token, err := NewTenantToken()
	if err != nil {
		return TenantKey{}, "", err
	}
	key := TenantKey{
		ID:        keyID,
		Label:     label,
		Hash:      HashTenantToken(token),
		CreatedAt: time.Now().UTC(),
	}
	tenant.Keys = append(tenant.Keys, key)
	if err := s.saveLocked(); err != nil {
		tenant.Keys = tenant.Keys[:len(tenant.Keys)-1]
		return TenantKey{}, "", err
	}
	return key, token, nil
}

// RevokeKey marks one key as revoked, keeping it for auditing.
func (s *TenantStore) RevokeKey(tenantID, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant := s.findLocked(tenantID)
	if tenant == nil {
		return errTenantNotFound
	}
	for i := range tenant.Keys {
		if tenant.Keys[i].ID != keyID {
			continue
		}
		if tenant.Keys[i].RevokedAt != nil {
			return nil
		}
		now := time.Now().UTC()
		tenant.Keys[i].RevokedAt = &now
		return s.saveLocked()
	}
	return errKeyNotFound
}

// Authenticate resolves a bearer token to its tenant principal.
func (s *TenantStore) Authenticate(token string) (principal, bool) {
	if token == "" {
		return principal{}, false
	}
	hash := HashTenantToken(token)

	s.mu.Lock()
	defer s.mu.Unlock()

	for ti := range s.file.Tenants {
		tenant := &s.file.Tenants[ti]
		for ki := range tenant.Keys {
			key := &tenant.Keys[ki]
			if key.RevokedAt != nil {
				continue
			}
			if subtle.ConstantTimeCompare([]byte(key.Hash), []byte(hash)) != 1 {
				continue
			}
			now := time.Now().UTC()
			if last, ok := s.lastSave[key.ID]; !ok || now.Sub(last) >= time.Minute {
				key.LastUsedAt = &now
				s.lastSave[key.ID] = now
				_ = s.saveLocked()
			}
			return principal{TenantID: tenant.ID, KeyID: key.ID}, true
		}
	}
	return principal{}, false
}

// AuthenticateAdmin checks a token against WHATSAPP_ADMIN_TOKEN.
func (s *TenantStore) AuthenticateAdmin(token string) bool {
	expected := os.Getenv("WHATSAPP_ADMIN_TOKEN")
	if expected == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

func (s *TenantStore) findLocked(id string) *Tenant {
	for i := range s.file.Tenants {
		if s.file.Tenants[i].ID == id {
			return &s.file.Tenants[i]
		}
	}
	return nil
}

// saveLocked rewrites tenants.json atomically. Callers must hold s.mu.
func (s *TenantStore) saveLocked() error {
	data, err := json.MarshalIndent(s.file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode tenants: %w", err)
	}
	temp, err := os.CreateTemp(s.root, ".tenants-*.json")
	if err != nil {
		return fmt.Errorf("failed to create tenant temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}
	if err := temp.Chmod(0600); err != nil {
		cleanup()
		return fmt.Errorf("failed to restrict tenant file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("failed to write tenant file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("failed to sync tenant file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("failed to close tenant file: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("failed to replace tenant file: %w", err)
	}
	return nil
}

func tenantView(tenant Tenant, status TenantStatus, jid string) TenantView {
	active := 0
	for _, key := range tenant.Keys {
		if key.RevokedAt == nil {
			active++
		}
	}
	return TenantView{
		ID:        tenant.ID,
		Label:     tenant.Label,
		Status:    status,
		JID:       jid,
		Keys:      active,
		CreatedAt: tenant.CreatedAt,
	}
}

func (t Tenant) hasKeyHash(hash string) bool {
	for _, key := range t.Keys {
		if key.Hash == hash {
			return true
		}
	}
	return false
}

func (t Tenant) hasKeyID(id string) bool {
	for _, key := range t.Keys {
		if key.ID == id {
			return true
		}
	}
	return false
}

func cloneTenant(tenant Tenant) Tenant {
	clone := tenant
	clone.Keys = append([]TenantKey(nil), tenant.Keys...)
	return clone
}
