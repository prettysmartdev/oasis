// Package nginx generates and applies NGINX configuration for the oasis gateway.
// Configuration is produced programmatically using go-crossplane AST types and
// applied via SIGHUP for graceful reload without dropping connections.
package nginx

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	crossplane "github.com/aluttik/go-crossplane"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// Configurator generates NGINX configuration from the app registry state
// and signals NGINX to reload when the configuration changes.
// Apply is safe to call from multiple goroutines concurrently.
type Configurator struct {
	mu         sync.Mutex
	configPath string
	nginxPID   func() (int, error)
}

// New creates a Configurator with default settings (/etc/nginx/nginx.conf and FindNginxPID).
func New() *Configurator {
	return NewWithConfig("/etc/nginx/nginx.conf", FindNginxPID)
}

// NewWithConfig creates a Configurator with explicit config path and PID finder.
// Pass an empty configPath to skip file writes (useful in tests).
func NewWithConfig(configPath string, nginxPID func() (int, error)) *Configurator {
	return &Configurator{
		configPath: configPath,
		nginxPID:   nginxPID,
	}
}

// FindNginxPID reads /var/run/nginx.pid and returns the NGINX master process PID.
func FindNginxPID() (int, error) {
	data, err := os.ReadFile("/var/run/nginx.pid")
	if err != nil {
		return 0, fmt.Errorf("read nginx.pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse nginx pid: %w", err)
	}
	return pid, nil
}

// Apply builds the NGINX config from apps and tailscaleIP, writes it atomically,
// and sends SIGHUP to trigger a graceful reload.
// If configPath is empty the write and SIGHUP steps are skipped (test mode).
// SIGHUP errors are logged but not returned — NGINX may not be running in dev.
// Concurrent calls are serialised by an internal mutex.
func (c *Configurator) Apply(_ context.Context, apps []db.App, tailscaleIP string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	config, err := buildConfig(apps, tailscaleIP)
	if err != nil {
		return fmt.Errorf("build nginx config: %w", err)
	}

	if c.configPath == "" {
		return nil
	}

	// Write atomically: temp file → rename.
	tmp := c.configPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(config), 0o644); err != nil {
		return fmt.Errorf("write nginx config: %w", err)
	}
	if err := os.Rename(tmp, c.configPath); err != nil {
		return fmt.Errorf("rename nginx config: %w", err)
	}

	// Send SIGHUP for graceful reload.
	pid, err := c.nginxPID()
	if err != nil {
		slog.Warn("could not find nginx PID, skipping reload", "err", err)
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		slog.Warn("could not find nginx process", "pid", pid, "err", err)
		return nil
	}
	if err := proc.Signal(syscall.SIGHUP); err != nil {
		slog.Warn("nginx SIGHUP failed", "pid", pid, "err", err)
	}
	return nil
}

// buildConfig constructs the full NGINX config string using the go-crossplane AST.
func buildConfig(apps []db.App, tailscaleIP string) (string, error) {
	// Build location blocks for enabled apps.
	var locations []crossplane.Directive
	for _, app := range apps {
		if !app.Enabled {
			continue
		}
		upstream := app.UpstreamURL
		if !strings.HasSuffix(upstream, "/") {
			upstream += "/"
		}
		loc := crossplane.Directive{
			Directive: "location",
			Args:      []string{"/apps/" + app.Slug + "/"},
			Block: &[]crossplane.Directive{
				{Directive: "proxy_pass", Args: []string{upstream}},
			},
		}
		locations = append(locations, loc)
	}

	// Fallback location for the static webapp.
	locations = append(locations, crossplane.Directive{
		Directive: "location",
		Args:      []string{"/"},
		Block: &[]crossplane.Directive{
			{Directive: "root", Args: []string{"/srv/webapp"}},
			{Directive: "try_files", Args: []string{"$uri", "$uri/", "/index.html"}},
		},
	})

	listenAddr := tailscaleIP + ":80"
	serverBlock := []crossplane.Directive{
		{Directive: "listen", Args: []string{listenAddr}},
	}
	serverBlock = append(serverBlock, locations...)

	parsed := []crossplane.Directive{
		{
			Directive: "events",
			Args:      []string{},
			Block:     &[]crossplane.Directive{},
		},
		{
			Directive: "http",
			Args:      []string{},
			Block: &[]crossplane.Directive{
				{
					Directive: "server",
					Args:      []string{},
					Block:     &serverBlock,
				},
			},
		},
	}

	cfg := crossplane.Config{
		File:   "",
		Parsed: parsed,
	}

	var buf bytes.Buffer
	if err := crossplane.Build(&buf, cfg, &crossplane.BuildOptions{Indent: 4}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
