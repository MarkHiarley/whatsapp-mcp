package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
	logger := waLog.Stdout("Client", "INFO", true)
	logger.Infof("Starting WhatsApp bridge...")

	apiToken := os.Getenv("WHATSAPP_API_TOKEN")
	if apiToken == "" {
		logger.Errorf("WHATSAPP_API_TOKEN is required")
		return
	}
	adminToken := os.Getenv("WHATSAPP_ADMIN_TOKEN")
	if adminToken == "" {
		logger.Warnf("WHATSAPP_ADMIN_TOKEN is not set; tenant provisioning is disabled")
	}

	storeDir := os.Getenv("WHATSAPP_STORE_DIR")
	if storeDir == "" {
		storeDir = "store"
	}
	port := 8080
	if rawPort := os.Getenv("PORT"); rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil || parsed < 1 || parsed > 65535 {
			logger.Errorf("Invalid PORT %q", rawPort)
			return
		}
		port = parsed
	}

	tenants, err := NewTenantStore(storeDir)
	if err != nil {
		logger.Errorf("Failed to initialize tenant store: %v", err)
		return
	}
	if err := tenants.EnsureDefault(apiToken); err != nil {
		logger.Errorf("Failed to initialize default tenant: %v", err)
		return
	}

	manager := NewTenantManager(tenants, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	manager.StartAll(ctx)
	startRESTServer(manager, port)

	logger.Infof("REST server is running on :%d. Press Ctrl+C to disconnect and exit.", port)
	<-ctx.Done()

	logger.Infof("Disconnecting...")
	manager.StopAll()
}
