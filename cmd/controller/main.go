// Package main is the entry point for the oasis controller binary.
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/prettysmartdev/oasis/internal/controller/api"
	"github.com/prettysmartdev/oasis/internal/controller/db"
	"github.com/prettysmartdev/oasis/internal/controller/health"
	"github.com/prettysmartdev/oasis/internal/controller/nginx"
	tsnetpkg "github.com/prettysmartdev/oasis/internal/controller/tsnet"
)

// version is embedded at build time via -ldflags "-X main.version=$(git describe --tags --always)".
var version = "dev"

// buildMgmtAddr returns the management API listen address for the given port.
// The host is always 127.0.0.1 — the management API must never bind to 0.0.0.0.
func buildMgmtAddr(port string) string {
	return "127.0.0.1:" + port
}

// envOrDefault returns the environment variable value or the default if unset.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// isTsnetConfigured reports whether the tsnet state directory contains state files,
// indicating the node was previously configured and can be started automatically.
func isTsnetConfigured(stateDir string) bool {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if ext == ".json" || e.Name() == "tailscaled.state" {
			return true
		}
	}
	return false
}

func main() {
	// 1. Read env vars.
	port := envOrDefault("OASIS_MGMT_PORT", "04515")
	hostname := envOrDefault("OASIS_HOSTNAME", "oasis")
	dbPath := envOrDefault("OASIS_DB_PATH", "/data/db/oasis.db")
	tsStateDir := envOrDefault("OASIS_TS_STATE_DIR", "/data/ts-state")
	logLevel := envOrDefault("OASIS_LOG_LEVEL", "info")

	level := slog.LevelInfo
	if logLevel == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 2. Open DB — ensure parent directory exists first (handles fresh volume mounts).
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		logger.Error("failed to create database directory", "path", filepath.Dir(dbPath), "err", err)
		os.Exit(1)
	}
	store, err := db.New(dbPath)
	if err != nil {
		logger.Error("failed to open database", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer store.Close()

	// 3. Create NGINX configurator.
	configurator := nginx.NewWithConfig("/etc/nginx/nginx.conf", nginx.FindNginxPID)

	// 4. Create tsnet node (not started yet — started by /api/v1/setup or on restart).
	node := tsnetpkg.NewNode(hostname, tsStateDir)

	// 5. Create API handlers.
	mgmtHandler := api.New(store, configurator, node, false)
	mgmtHandler.SetVersion(version)

	tsnetHandler := api.New(store, configurator, node, true)
	tsnetHandler.SetVersion(version)

	// 6. Start management API server (loopback only).
	mgmtAddr := buildMgmtAddr(port)
	ln, err := net.Listen("tcp", mgmtAddr)
	if err != nil {
		logger.Error("failed to bind management API",
			"addr", mgmtAddr,
			"err", err,
			"hint", "is another instance running? Change OASIS_MGMT_PORT or run 'oasis stop'",
		)
		os.Exit(1)
	}

	mgmtMux := http.NewServeMux()
	mgmtHandler.RegisterRoutes(mgmtMux)
	mgmtServer := &http.Server{Addr: mgmtAddr, Handler: mgmtMux}

	// Note: the management API begins accepting requests here, before the NGINX config
	// is applied in step 7. In practice the window is a few milliseconds; a strict
	// "apply-before-accept" ordering would require holding Accept until after Apply,
	// adding complexity for negligible real-world benefit.
	go func() {
		if err := mgmtServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("management server error", "err", err)
			stop()
		}
	}()
	logger.Info("oasis controller started", "version", version, "mgmt", mgmtAddr)

	// 7. If tsnet state already exists, start the node and serve the webapp API.
	var tsnetServer *http.Server
	if isTsnetConfigured(tsStateDir) {
		tsLn, err := node.Start(ctx)
		if err != nil {
			logger.Warn("failed to start tsnet node (will retry via /api/v1/setup)", "err", err)
		} else {
			// Apply current NGINX config now that we have a Tailscale IP.
			apps, _ := store.ListApps(ctx)
			if ip, err := node.TailscaleIP(); err == nil && ip != "" {
				if err := configurator.Apply(ctx, apps, ip); err != nil {
					logger.Warn("failed to apply nginx config", "err", err)
				}
			}

			// Serve the read-only webapp API on the tsnet listener.
			tsnetMux := http.NewServeMux()
			tsnetHandler.RegisterRoutes(tsnetMux)
			tsnetServer = &http.Server{Handler: tsnetMux}
			go func() {
				if err := tsnetServer.Serve(tsLn); err != nil && err != http.ErrServerClosed {
					logger.Error("tsnet server error", "err", err)
				}
			}()
			if ip, _ := node.TailscaleIP(); ip != "" {
				logger.Info("tsnet webapp API started", "ip", ip)
			}
		}
	}

	// 8. Start background health checker.
	checker := health.New(store, 30*time.Second)
	go checker.Start(ctx)

	// 9. Block until shutdown signal; graceful shutdown in reverse order.
	<-ctx.Done()
	logger.Info("shutting down")

	if tsnetServer != nil {
		if err := tsnetServer.Close(); err != nil {
			logger.Error("error closing tsnet server", "err", err)
		}
	}
	if err := mgmtServer.Close(); err != nil {
		logger.Error("error closing management server", "err", err)
	}
	if err := node.Close(); err != nil {
		logger.Error("error closing tsnet node", "err", err)
	}
}
