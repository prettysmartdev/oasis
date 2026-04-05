// Package main is the entry point for the oasis controller binary.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// version is embedded at build time via -ldflags "-X main.version=$(git describe --tags --always)".
var version = "dev"

// buildMgmtAddr returns the management API listen address for the given port.
// The host is always 127.0.0.1 — the management API must never bind to 0.0.0.0.
func buildMgmtAddr(port string) string {
	return "127.0.0.1:" + port
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	port := os.Getenv("OASIS_MGMT_PORT")
	if port == "" {
		port = "04515"
	}

	mgmtAddr := buildMgmtAddr(port)

	mgmtMux := http.NewServeMux()
	mgmtMux.HandleFunc("/", notImplementedHandler)

	// TODO: start tsnet-facing API server on the Tailscale interface (future work item).

	mgmtServer := &http.Server{
		Addr:    mgmtAddr,
		Handler: mgmtMux,
	}

	// Attempt to bind early to detect port conflicts before logging "started".
	ln, err := net.Listen("tcp", mgmtAddr)
	if err != nil {
		logger.Error("failed to bind management API",
			"addr", mgmtAddr,
			"err", err,
			"hint", "is another instance running? Change OASIS_MGMT_PORT or run 'oasis stop'",
		)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("oasis controller started", "version", version, "mgmt", mgmtAddr)

	go func() {
		if err := mgmtServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("management server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	if err := mgmtServer.Close(); err != nil {
		logger.Error("error closing management server", "err", err)
	}
}

// notImplementedHandler returns 501 Not Implemented for all routes.
func notImplementedHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	// Encode errors after WriteHeader cannot change the response code; log and continue.
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented",
		"code":  "NOT_IMPLEMENTED",
	}); err != nil {
		slog.Default().Error("failed to write response body", "err", err)
	}
}
