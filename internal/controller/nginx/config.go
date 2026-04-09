// Package nginx generates and applies NGINX configuration for the oasis gateway.
// Configuration is produced programmatically using go-crossplane AST types and
// applied via SIGHUP for graceful reload without dropping connections.
//
// NGINX listens on LocalAddr (127.0.0.1:8080) inside the container — not on the
// Tailscale IP, which only exists in tsnet's userspace Go network stack and is
// therefore inaccessible to other OS processes. Tailnet traffic arrives at the
// tsnet Go HTTP server, which reverse-proxies non-API requests to LocalAddr.
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

// LocalAddr is the local address NGINX listens on inside the container.
// The tsnet Go HTTP server proxies incoming Tailscale traffic to this address.
const LocalAddr = "http://127.0.0.1:8080"

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

// FindNginxPID reads /tmp/nginx.pid and returns the NGINX master process PID.
func FindNginxPID() (int, error) {
	data, err := os.ReadFile("/tmp/nginx.pid")
	if err != nil {
		return 0, fmt.Errorf("read nginx.pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse nginx pid: %w", err)
	}
	return pid, nil
}

// Apply builds the NGINX config from apps, writes it atomically, and sends SIGHUP
// to trigger a graceful reload.
// If configPath is empty the write and SIGHUP steps are skipped (test mode).
// SIGHUP errors are logged but not returned — NGINX may not be running in dev.
// Concurrent calls are serialised by an internal mutex.
func (c *Configurator) Apply(_ context.Context, apps []db.App) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	config, err := buildConfig(apps)
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

// proxyHeaders returns the NGINX directives to forward standard proxy request headers
// and strip response headers that would prevent iFrame embedding in the oasis dashboard.
//
// X-Frame-Options and Content-Security-Policy are removed from upstream responses so
// the browser will not refuse to embed the app in the dashboard iFrame. Note: stripping
// Content-Security-Policy weakens the upstream app's security policy in the browser —
// this is an explicit trade-off for the iFrame proxy experience.
func proxyHeaders() []crossplane.Directive {
	return []crossplane.Directive{
		{Directive: "proxy_set_header", Args: []string{"Host", "$host"}},
		{Directive: "proxy_set_header", Args: []string{"X-Real-IP", "$remote_addr"}},
		{Directive: "proxy_set_header", Args: []string{"X-Forwarded-For", "$proxy_add_x_forwarded_for"}},
		{Directive: "proxy_set_header", Args: []string{"X-Forwarded-Proto", "$scheme"}},
		{Directive: "proxy_hide_header", Args: []string{"X-Frame-Options"}},
		{Directive: "proxy_hide_header", Args: []string{"Content-Security-Policy"}},
	}
}

// buildConfig constructs the full NGINX config string using the go-crossplane AST.
func buildConfig(apps []db.App) (string, error) {
	// Build location blocks for enabled apps with proxy access type.
	// Direct apps are opened in a new browser tab by the dashboard and do not
	// require an NGINX route — skipping their location block is intentional and
	// does not affect the health-check loop (which probes upstreamURL directly).
	//
	// Known limitation: upstream apps that hard-code the root "/" path in asset
	// references (e.g. <script src="/static/main.js">) will break when served
	// under the path prefix /apps/<slug>/. Those assets will be requested as
	// /static/main.js instead of /apps/<slug>/static/main.js. This is an
	// inherent limitation of path-prefix proxying; no sub_filter workaround is
	// applied here.
	var locations []crossplane.Directive
	for _, app := range apps {
		if !app.Enabled || app.AccessType != "proxy" {
			continue
		}
		upstream := app.UpstreamURL
		if !strings.HasSuffix(upstream, "/") {
			upstream += "/"
		}
		block := []crossplane.Directive{
			{Directive: "proxy_pass", Args: []string{upstream}},
		}
		block = append(block, proxyHeaders()...)
		loc := crossplane.Directive{
			Directive: "location",
			Args:      []string{"/apps/" + app.Slug + "/"},
			Block:     &block,
		}
		locations = append(locations, loc)
	}

	// Service worker — must not be cached so browsers always re-validate on load.
	locations = append(locations, crossplane.Directive{
		Directive: "location",
		Args:      []string{"=", "/sw.js"},
		Block: &[]crossplane.Directive{
			{Directive: "root", Args: []string{"/srv/webapp"}},
			{Directive: "add_header", Args: []string{"Cache-Control", `"no-store, no-cache, must-revalidate"`}},
		},
	})

	// Fallback location for the static webapp.
	locations = append(locations, crossplane.Directive{
		Directive: "location",
		Args:      []string{"/"},
		Block: &[]crossplane.Directive{
			{Directive: "root", Args: []string{"/srv/webapp"}},
			{Directive: "try_files", Args: []string{"$uri", "$uri/", "/index.html"}},
		},
	})

	// NGINX listens on a fixed local address. Tailnet traffic arrives via the
	// tsnet Go HTTP server, which reverse-proxies non-API requests here.
	// Using a local port avoids the EADDRNOTAVAIL error that occurs when NGINX
	// tries to bind to the Tailscale IP (which only exists in tsnet's userspace
	// network stack and is not visible to other OS processes).
	listenAddr := "127.0.0.1:8080"
	serverBlock := []crossplane.Directive{
		{Directive: "listen", Args: []string{listenAddr}},
	}
	serverBlock = append(serverBlock, locations...)

	parsed := []crossplane.Directive{
		{Directive: "pid", Args: []string{"/tmp/nginx.pid"}},
		{Directive: "error_log", Args: []string{"/dev/stderr", "warn"}},
		{
			Directive: "events",
			Args:      []string{},
			Block:     &[]crossplane.Directive{},
		},
		{
			Directive: "http",
			Args:      []string{},
			Block: &[]crossplane.Directive{
				{Directive: "include", Args: []string{"/etc/nginx/mime.types"}},
				{Directive: "default_type", Args: []string{"application/octet-stream"}},
				{Directive: "access_log", Args: []string{"/dev/stdout"}},
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
