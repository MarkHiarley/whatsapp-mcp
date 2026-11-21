package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// qrEvent is one SSE payload of the pairing stream.
type qrEvent struct {
	Kind   string
	Code   string
	Status TenantStatus
}

type tenantRuntime struct {
	mu           sync.RWMutex
	tenant       Tenant
	container    *sqlstore.Container
	client       *whatsmeow.Client
	store        *MessageStore
	status       TenantStatus
	pairing      bool
	reconnecting bool
	qrSubs       map[chan qrEvent]struct{}
	logger       waLog.Logger
}

// TenantManager owns one whatsmeow session and one message store per tenant.
type TenantManager struct {
	mu          sync.RWMutex
	runtimes    map[string]*tenantRuntime
	tenants     *TenantStore
	logger      waLog.Logger
	adminToken  string
	stop        chan struct{}
	stopOnce    sync.Once
	eventOnce   sync.Once
	eventCancel context.CancelFunc
}

// NewTenantManager wires the registry to the in-process session manager.
func NewTenantManager(tenants *TenantStore, logger waLog.Logger) *TenantManager {
	if logger == nil {
		logger = waLog.Noop
	}
	return &TenantManager{
		runtimes:   make(map[string]*tenantRuntime),
		tenants:    tenants,
		logger:     logger,
		adminToken: os.Getenv("WHATSAPP_ADMIN_TOKEN"),
		stop:       make(chan struct{}),
	}
}

// TenantStore exposes the registry backing the API.
func (m *TenantManager) TenantStore() *TenantStore {
	return m.tenants
}

// AdminEnabled reports whether an admin token was configured.
func (m *TenantManager) AdminEnabled() bool {
	return m.adminToken != ""
}

// AuthenticateAdmin verifies an admin bearer token in constant time.
func (m *TenantManager) AuthenticateAdmin(token string) bool {
	if token == "" || m.adminToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(m.adminToken)) == 1
}

// StartAll opens every registered tenant without blocking the boot.
func (m *TenantManager) StartAll(ctx context.Context) {
	m.StartEventWorker(ctx)
	for _, view := range m.tenants.List() {
		tenant, ok := m.tenants.Get(view.ID)
		if !ok {
			continue
		}
		rt, err := m.newRuntime(tenant)
		if err != nil {
			m.logger.Errorf("Failed to initialize tenant %s: %v", tenant.ID, err)
			continue
		}
		if !m.addRuntime(rt) {
			_ = rt.store.Close()
			continue
		}
		go func(rt *tenantRuntime) {
			if err := m.StartRuntime(rt); err != nil {
				m.logger.Errorf("Tenant %s failed to start: %v", rt.tenant.ID, err)
			}
		}(rt)
	}
}

// CreateTenant registers a tenant and starts its session.
func (m *TenantManager) CreateTenant(id, label string) (Tenant, string, string, error) {
	tenant, keyID, token, err := m.tenants.Create(id, label)
	if err != nil {
		return Tenant{}, "", "", err
	}
	rt, err := m.newRuntime(tenant)
	if err != nil {
		_ = m.tenants.Delete(id)
		return Tenant{}, "", "", err
	}
	if !m.addRuntime(rt) {
		_ = rt.store.Close()
		_ = m.tenants.Delete(id)
		return Tenant{}, "", "", errTenantExists
	}
	go func() {
		if err := m.StartRuntime(rt); err != nil {
			m.logger.Errorf("Tenant %s failed to start: %v", id, err)
		}
	}()
	return tenant, keyID, token, nil
}

// DeleteTenant disconnects the session and forgets the tenant. Files stay.
func (m *TenantManager) DeleteTenant(id string) error {
	rt, ok := m.Runtime(id)
	if ok {
		if rt.client != nil {
			rt.client.Disconnect()
		}
		m.mu.Lock()
		delete(m.runtimes, id)
		m.mu.Unlock()
		if rt.store != nil {
			_ = rt.store.Close()
		}
		if rt.container != nil {
			_ = rt.container.Close()
		}
	}
	if err := m.tenants.Delete(id); err != nil {
		if ok {
			return nil
		}
		return err
	}
	return nil
}

// Runtime returns the live runtime of one tenant.
func (m *TenantManager) Runtime(id string) (*tenantRuntime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt, ok := m.runtimes[id]
	return rt, ok
}

// Views lists every tenant with its live status.
func (m *TenantManager) Views() []TenantView {
	views := m.tenants.List()
	for i := range views {
		if rt, ok := m.Runtime(views[i].ID); ok {
			views[i] = rt.View()
		}
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views
}

// StartRuntime opens the tenant SQLite session, builds the client and connects.
func (m *TenantManager) StartRuntime(rt *tenantRuntime) error {
	rt.mu.RLock()
	if rt.client != nil {
		rt.mu.RUnlock()
		return nil
	}
	tenant := rt.tenant
	rt.mu.RUnlock()

	if !ValidTenantID(tenant.ID) {
		return errInvalidTenantID
	}
	paths := pathsForTenant(m.tenants.Root(), tenant.ID)
	ctx := context.Background()

	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New(ctx, "sqlite3", sessionDSN(paths.sessionDB), dbLog)
	if err != nil {
		m.setStatus(rt, TenantDisconnected)
		return fmt.Errorf("failed to open tenant session database: %w", err)
	}

	device, err := container.GetFirstDevice(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		device = container.NewDevice()
	case err != nil:
		_ = container.Close()
		m.setStatus(rt, TenantDisconnected)
		return fmt.Errorf("failed to load tenant device: %w", err)
	}

	client := whatsmeow.NewClient(device, rt.logger)
	if client == nil {
		_ = container.Close()
		m.setStatus(rt, TenantDisconnected)
		return fmt.Errorf("failed to create tenant client")
	}

	rt.mu.Lock()
	rt.container = container
	rt.client = client
	rt.mu.Unlock()

	m.attachHandlers(rt)

	if device.ID == nil {
		if _, err := m.startQRChannel(rt, client, false); err != nil {
			m.setStatus(rt, TenantDisconnected)
			return err
		}
		return nil
	}

	if err := client.Connect(); err != nil {
		m.setStatus(rt, TenantDisconnected)
		m.scheduleReconnect(rt)
		return fmt.Errorf("failed to connect tenant %s: %w", tenant.ID, err)
	}
	return nil
}

// SubscribeQR registers one SSE client for the tenant pairing stream.
func (m *TenantManager) SubscribeQR(rt *tenantRuntime) <-chan qrEvent {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.addSubLocked()
}

// UnsubscribeQR removes and closes one SSE client channel.
func (m *TenantManager) UnsubscribeQR(rt *tenantRuntime, ch <-chan qrEvent) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for sub := range rt.qrSubs {
		if (<-chan qrEvent)(sub) == ch {
			delete(rt.qrSubs, sub)
			close(sub)
			return
		}
	}
}

// StartPairing begins (or joins) the QR flow and returns its event stream.
func (m *TenantManager) StartPairing(rt *tenantRuntime) (<-chan qrEvent, error) {
	rt.mu.Lock()
	if rt.client == nil || rt.container == nil {
		rt.mu.Unlock()
		return nil, fmt.Errorf("tenant runtime is not started")
	}
	if rt.status == TenantConnected {
		ch := make(chan qrEvent, 1)
		ch <- qrEvent{Kind: "status", Status: TenantConnected}
		close(ch)
		rt.mu.Unlock()
		return ch, nil
	}
	if rt.pairing {
		ch := rt.addSubLocked()
		ch <- qrEvent{Kind: "status", Status: rt.status}
		rt.mu.Unlock()
		return ch, nil
	}
	if rt.status == TenantLoggedOut && rt.client.Store.ID != nil {
		if err := rt.resetClientLocked(); err != nil {
			rt.mu.Unlock()
			return nil, err
		}
		m.attachHandlers(rt)
	}
	client := rt.client
	rt.mu.Unlock()

	ch, err := m.startQRChannel(rt, client, true)
	if err != nil {
		if client.IsConnected() {
			m.setStatus(rt, TenantConnected)
			status := make(chan qrEvent, 1)
			status <- qrEvent{Kind: "status", Status: TenantConnected}
			close(status)
			return status, nil
		}
		return nil, err
	}
	return ch, nil
}

// StopAll disconnects every tenant and releases its stores.
func (m *TenantManager) StopAll() {
	m.stopOnce.Do(func() { close(m.stop) })
	m.mu.Lock()
	if m.eventCancel != nil {
		m.eventCancel()
		m.eventCancel = nil
	}
	runtimes := make([]*tenantRuntime, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		runtimes = append(runtimes, rt)
	}
	m.mu.Unlock()

	for _, rt := range runtimes {
		if rt.client != nil {
			rt.client.Disconnect()
		}
		if rt.store != nil {
			_ = rt.store.Close()
		}
		if rt.container != nil {
			_ = rt.container.Close()
		}
	}
}

func (m *TenantManager) newRuntime(tenant Tenant) (*tenantRuntime, error) {
	if !ValidTenantID(tenant.ID) {
		return nil, errInvalidTenantID
	}
	paths := pathsForTenant(m.tenants.Root(), tenant.ID)
	store, err := NewMessageStoreAt(paths.messagesDB, paths.mediaDir)
	if err != nil {
		return nil, err
	}
	return &tenantRuntime{
		tenant: tenant,
		store:  store,
		status: TenantDisconnected,
		qrSubs: make(map[chan qrEvent]struct{}),
		logger: m.logger,
	}, nil
}

func (m *TenantManager) addRuntime(rt *tenantRuntime) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.runtimes[rt.tenant.ID]; exists {
		return false
	}
	m.runtimes[rt.tenant.ID] = rt
	return true
}

func (m *TenantManager) attachHandlers(rt *tenantRuntime) {
	rt.client.AddEventHandler(func(evt any) {
		switch v := evt.(type) {
		case *events.Message:
			handleMessage(rt, v, rt.logger)
		case *events.HistorySync:
			handleHistorySync(rt, v, rt.logger)
		case *events.Connected:
			m.setStatus(rt, TenantConnected)
			rt.mu.RLock()
			client := rt.client
			store := rt.store
			rt.mu.RUnlock()
			if client != nil && store != nil {
				go func() {
					if err := store.SyncContacts(client); err != nil {
						rt.logger.Warnf("Failed to sync contacts for tenant %s: %v", rt.tenant.ID, err)
					}
				}()
			}
		case *events.LoggedOut:
			rt.logger.Warnf("Tenant %s device logged out, a new QR pairing is required", rt.tenant.ID)
			m.setStatus(rt, TenantLoggedOut)
			if rt.client != nil {
				rt.client.Disconnect()
			}
		case *events.Disconnected:
			m.setStatus(rt, TenantDisconnected)
			m.scheduleReconnect(rt)
		case *events.ConnectFailure:
			m.setStatus(rt, TenantDisconnected)
			m.scheduleReconnect(rt)
		case *events.KeepAliveTimeout:
			m.setStatus(rt, TenantDisconnected)
			m.scheduleReconnect(rt)
		case *events.KeepAliveRestored:
			m.setStatus(rt, TenantConnected)
		case *events.StreamReplaced:
			// Another client took over this session; reconnecting would fight it.
			m.setStatus(rt, TenantDisconnected)
			rt.logger.Warnf("Tenant %s session was replaced by another client", rt.tenant.ID)
		}
	})
}

// scheduleReconnect retries the websocket with exponential backoff while the
// tenant has a paired device and the process is still running.
func (m *TenantManager) scheduleReconnect(rt *tenantRuntime) {
	rt.mu.Lock()
	if rt.reconnecting || rt.pairing || rt.client == nil || rt.status == TenantLoggedOut ||
		rt.client.Store == nil || rt.client.Store.ID == nil {
		rt.mu.Unlock()
		return
	}
	rt.reconnecting = true
	rt.mu.Unlock()

	go func() {
		defer func() {
			rt.mu.Lock()
			rt.reconnecting = false
			rt.mu.Unlock()
		}()

		backoff := 2 * time.Second
		for {
			select {
			case <-m.stop:
				return
			case <-time.After(backoff):
			}

			rt.mu.RLock()
			client := rt.client
			status := rt.status
			pairing := rt.pairing
			rt.mu.RUnlock()
			if client == nil || status == TenantConnected || status == TenantLoggedOut || pairing {
				return
			}
			if client.Store == nil || client.Store.ID == nil {
				return
			}
			if client.IsConnected() {
				m.setStatus(rt, TenantConnected)
				return
			}
			if err := client.Connect(); err != nil {
				rt.logger.Warnf("Tenant %s reconnect failed: %v", rt.tenant.ID, err)
				if backoff < 5*time.Minute {
					backoff *= 2
				}
				continue
			}
			return
		}
	}()
}

// startQRChannel opens the QR channel before Connect and consumes it.
func (m *TenantManager) startQRChannel(rt *tenantRuntime, client *whatsmeow.Client, subscribe bool) (<-chan qrEvent, error) {
	ctx := context.Background()
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return nil, err
	}

	rt.mu.Lock()
	rt.status = TenantPairing
	rt.pairing = true
	var ch chan qrEvent
	if subscribe {
		ch = rt.addSubLocked()
	}
	rt.mu.Unlock()

	if err := client.Connect(); err != nil {
		rt.mu.Lock()
		rt.pairing = false
		rt.status = TenantDisconnected
		if ch != nil {
			delete(rt.qrSubs, ch)
			close(ch)
		}
		rt.mu.Unlock()
		return nil, err
	}

	m.broadcast(rt, qrEvent{Kind: "status", Status: TenantPairing})
	if ch != nil {
		ch <- qrEvent{Kind: "status", Status: TenantPairing}
	}
	go m.consumeQR(rt, qrChan)
	return ch, nil
}

func (m *TenantManager) consumeQR(rt *tenantRuntime, qrChan <-chan whatsmeow.QRChannelItem) {
	for evt := range qrChan {
		switch evt.Event {
		case whatsmeow.QRChannelEventCode:
			m.broadcast(rt, qrEvent{Kind: "qr", Code: evt.Code, Status: TenantPairing})
			if rt.tenant.ID == LegacyTenantID {
				fmt.Println("\nScan this QR code with your WhatsApp app:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			}
		case whatsmeow.QRChannelSuccess.Event:
			m.setStatus(rt, TenantConnected)
		case whatsmeow.QRChannelTimeout.Event:
			m.setStatus(rt, TenantDisconnected)
		case "error":
			if evt.Error != nil {
				rt.logger.Warnf("Tenant %s pairing error: %v", rt.tenant.ID, evt.Error)
			}
			m.setStatus(rt, TenantDisconnected)
		default:
			rt.logger.Infof("Tenant %s QR event: %s", rt.tenant.ID, evt.Event)
		}
	}

	rt.mu.Lock()
	rt.pairing = false
	status := rt.status
	rt.mu.Unlock()
	if status == TenantPairing {
		m.setStatus(rt, TenantDisconnected)
	}
}

func (m *TenantManager) setStatus(rt *tenantRuntime, status TenantStatus) {
	rt.mu.Lock()
	rt.status = status
	if status != TenantPairing {
		rt.pairing = false
	}
	rt.mu.Unlock()
	m.broadcast(rt, qrEvent{Kind: "status", Status: status})
}

func (m *TenantManager) broadcast(rt *tenantRuntime, ev qrEvent) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	for ch := range rt.qrSubs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (rt *tenantRuntime) addSubLocked() chan qrEvent {
	ch := make(chan qrEvent, 8)
	rt.qrSubs[ch] = struct{}{}
	return ch
}

// View projects the runtime into the public tenant shape.
func (rt *tenantRuntime) View() TenantView {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	jid := ""
	if rt.client != nil && rt.client.Store != nil && rt.client.Store.ID != nil {
		jid = rt.client.Store.ID.String()
	}
	return tenantView(rt.tenant, rt.status, jid)
}

// Status reads the current connection state.
func (rt *tenantRuntime) Status() TenantStatus {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.status
}

// resetClientLocked drops a logged-out device and creates a fresh client.
// The caller must hold rt.mu.
func (rt *tenantRuntime) resetClientLocked() error {
	if rt.client != nil {
		rt.client.Disconnect()
		if rt.client.Store != nil && rt.client.Store.ID != nil && rt.container != nil {
			_ = rt.container.DeleteDevice(context.Background(), rt.client.Store)
		}
	}
	if rt.container == nil {
		return fmt.Errorf("tenant container is not initialized")
	}
	device := rt.container.NewDevice()
	if device == nil {
		return fmt.Errorf("failed to create a new tenant device")
	}
	rt.client = whatsmeow.NewClient(device, rt.logger)
	if rt.client == nil {
		return fmt.Errorf("failed to create a new tenant client")
	}
	rt.status = TenantDisconnected
	return nil
}

type tenantPaths struct {
	sessionDB  string
	messagesDB string
	mediaDir   string
}

// pathsForTenant maps a tenant to its on-disk layout.
func pathsForTenant(root, id string) tenantPaths {
	if id == LegacyTenantID {
		return tenantPaths{
			sessionDB:  filepath.Join(root, "whatsapp.db"),
			messagesDB: filepath.Join(root, "messages.db"),
			mediaDir:   root,
		}
	}
	base := filepath.Join(root, "tenants", id)
	return tenantPaths{
		sessionDB:  filepath.Join(base, "whatsapp.db"),
		messagesDB: filepath.Join(base, "messages.db"),
		mediaDir:   filepath.Join(base, "media"),
	}
}

func sessionDSN(path string) string {
	return "file:" + path + "?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL"
}
