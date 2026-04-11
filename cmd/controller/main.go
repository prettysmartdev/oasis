// Package main is the entry point for the oasis controller binary.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	agentpkg "github.com/prettysmartdev/oasis/internal/controller/agent"
	claudepkg "github.com/prettysmartdev/oasis/internal/controller/agent/claude"
	"github.com/prettysmartdev/oasis/internal/controller/api"
	"github.com/prettysmartdev/oasis/internal/controller/db"
	"github.com/prettysmartdev/oasis/internal/controller/health"
	"github.com/prettysmartdev/oasis/internal/controller/nginx"
	tsnetpkg "github.com/prettysmartdev/oasis/internal/controller/tsnet"
)

// version is embedded at build time via -ldflags "-X main.version=$(git describe --tags --always)".
var version = "dev"

// buildMgmtAddr returns the management API listen address for the given host and port.
// Inside a Docker container the default host is 0.0.0.0 so Docker's port forwarding
// can reach the process; host-side security is enforced by the -p 127.0.0.1:... binding
// in the docker run command. For direct (non-Docker) execution set OASIS_MGMT_HOST=127.0.0.1.
func buildMgmtAddr(host, port string) string {
	return host + ":" + port
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
	mgmtHost := envOrDefault("OASIS_MGMT_HOST", "0.0.0.0")
	hostname := envOrDefault("OASIS_HOSTNAME", "oasis")
	dbPath := envOrDefault("OASIS_DB_PATH", "/data/db/oasis.db")
	tsStateDir := envOrDefault("OASIS_TS_STATE_DIR", "/data/ts-state")
	logLevel := envOrDefault("OASIS_LOG_LEVEL", "info")
	tsAPIKey := os.Getenv("TAILSCALE_API_KEY") // optional; enables automatic conflict resolution
	runsDir := envOrDefault("OASIS_AGENT_RUNS_DIR", "/data/agent-runs")
	claudeBin := os.Getenv("OASIS_CLAUDE_BIN")
	chatTimeoutStr := envOrDefault("OASIS_CHAT_TIMEOUT", "120s")

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
	logger.Info("database opened", "path", dbPath)

	// Create agent runs directory.
	if err := os.MkdirAll(runsDir, 0o750); err != nil {
		logger.Error("failed to create agent runs directory", "path", runsDir, "err", err)
		os.Exit(1)
	}
	logger.Info("agent runs directory ready", "path", runsDir)

	// 3. Create NGINX configurator.
	configurator := nginx.NewWithConfig("/etc/nginx/nginx.conf", nginx.FindNginxPID)

	// 4. Create tsnet node (not started yet — started by /api/v1/setup or on restart).
	node := tsnetpkg.NewNode(hostname, tsStateDir)
	if tsAPIKey != "" {
		node.SetTailscaleAPIKey(tsAPIKey)
	}

	// 5. Create API handlers.
	mgmtHandler := api.New(store, configurator, node, false)
	mgmtHandler.SetVersion(version)

	tsnetHandler := api.New(store, configurator, node, true)
	tsnetHandler.SetVersion(version)

	// startTsnetServer begins serving the read-only webapp API on the given tsnet listener.
	// Called both from the auto-start path (step 7) and from the first-run setup callback.
	startTsnetServer := func(tsLn net.Listener) {
		tsnetMux := http.NewServeMux()
		tsnetHandler.RegisterRoutes(tsnetMux)
		go func() {
			srv := &http.Server{Handler: tsnetMux}
			if err := srv.Serve(tsLn); err != nil && err != http.ErrServerClosed {
				logger.Error("tsnet server error", "err", err)
			}
		}()
		if dnsName, err := node.TailscaleDNSName(context.Background()); err == nil {
			logger.Info("tsnet webapp API started", "url", "https://"+dnsName)
		} else if ip, err := node.TailscaleIP(); err == nil && ip != "" {
			logger.Info("tsnet webapp API started", "ip", ip)
		}
		apps, _ := store.ListApps(context.Background())
		if applyErr := configurator.Apply(context.Background(), apps); applyErr != nil {
			logger.Warn("failed to apply nginx config", "err", applyErr)
		} else {
			logger.Info("nginx config applied")
		}
	}

	// Register the setup callback so first-run setup (via POST /api/v1/setup) also
	// starts the webapp API server without requiring a container restart.
	mgmtHandler.SetOnSetup(startTsnetServer)

	claudeHarness := claudepkg.New(claudeBin)

	// Parse chat timeout.
	chatTimeout, err := time.ParseDuration(chatTimeoutStr)
	if err != nil {
		chatTimeout = 120 * time.Second
	}

	mgmtHandler.SetHarness(claudeHarness)
	mgmtHandler.SetRunsDir(runsDir)
	mgmtHandler.SetChatTimeout(chatTimeout)
	// tsnetHandler gets the same harness/runsDir for consistency (webhook triggers are on tsnet too).
	tsnetHandler.SetHarness(claudeHarness)
	tsnetHandler.SetRunsDir(runsDir)
	tsnetHandler.SetChatTimeout(chatTimeout)

	// 6. Start management API server (loopback only).
	mgmtAddr := buildMgmtAddr(mgmtHost, port)
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

	// 7. If tsnet state already exists, reconnect and serve the webapp API.
	if isTsnetConfigured(tsStateDir) {
		logger.Info("tsnet state found, starting node", "hostname", hostname)
		tsLn, err := node.Start(ctx)
		if err != nil {
			var conflictErr *tsnetpkg.HostnameConflictError
			if errors.As(err, &conflictErr) {
				// A hostname conflict means the status API and webapp would
				// report the wrong hostname. Exit so the operator can resolve
				// the conflict rather than silently running with a wrong name.
				logger.Error("hostname conflict — exiting", "err", conflictErr)
				os.Exit(1)
			}
			logger.Warn("failed to start tsnet node (will retry via /api/v1/setup)", "err", err)
		} else {
			startTsnetServer(tsLn)
		}
	} else {
		logger.Info("tsnet not yet configured — run oasis init to set up")
	}

	// 8. Start background health checker.
	checker := health.New(store, 30*time.Second)
	go checker.Start(ctx)
	logger.Info("health checker started", "interval", 30*time.Second)

	// 8b. Start agent scheduler.
	scheduler := agentpkg.NewScheduler(store, claudeHarness, runsDir)
	go scheduler.Start(ctx)
	logger.Info("agent scheduler started")

	// 9. Block until shutdown signal; graceful shutdown in reverse order.
	<-ctx.Done()
	logger.Info("shutting down")

	if err := mgmtServer.Close(); err != nil {
		logger.Error("error closing management server", "err", err)
	}
	if err := node.Close(); err != nil {
		logger.Error("error closing tsnet node", "err", err)
	}
}
