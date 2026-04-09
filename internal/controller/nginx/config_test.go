package nginx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// nopPID returns an error so that SIGHUP is always skipped in tests.
func nopPID() (int, error) {
	return 0, fmt.Errorf("no nginx in test")
}

// TestApplyNoApps verifies that an empty app list produces a config with no "/apps/" blocks.
func TestApplyNoApps(t *testing.T) {
	config, err := buildConfig([]db.App{})
	if err != nil {
		t.Fatalf("buildConfig error: %v", err)
	}
	if strings.Contains(config, "/apps/") {
		t.Errorf("expected no /apps/ location blocks in config, got:\n%s", config)
	}
}

// TestApplyDisabledApp verifies that a disabled app does not produce a location block.
func TestApplyDisabledApp(t *testing.T) {
	apps := []db.App{
		{
			Slug:        "disabled-app",
			UpstreamURL: "http://localhost:9000",
			Enabled:     false,
		},
	}

	config, err := buildConfig(apps)
	if err != nil {
		t.Fatalf("buildConfig error: %v", err)
	}
	if strings.Contains(config, "/apps/disabled-app") {
		t.Errorf("expected no location block for disabled app, got:\n%s", config)
	}
}

// TestApplyEnabledApp verifies that an enabled proxy app produces a proxy_pass location block.
func TestApplyEnabledApp(t *testing.T) {
	apps := []db.App{
		{
			Slug:        "myapp",
			UpstreamURL: "http://localhost:3000",
			Enabled:     true,
			AccessType:  "proxy",
		},
	}

	config, err := buildConfig(apps)
	if err != nil {
		t.Fatalf("buildConfig error: %v", err)
	}
	if !strings.Contains(config, "proxy_pass") {
		t.Errorf("expected proxy_pass directive in config, got:\n%s", config)
	}
	if !strings.Contains(config, "http://localhost:3000/") {
		t.Errorf("expected upstream URL 'http://localhost:3000/' in config, got:\n%s", config)
	}
	if !strings.Contains(config, "/apps/myapp/") {
		t.Errorf("expected '/apps/myapp/' location in config, got:\n%s", config)
	}
}

// TestDirectAppNoLocationBlock verifies that an enabled app with AccessType="direct"
// does not produce a location block in the NGINX config.
func TestDirectAppNoLocationBlock(t *testing.T) {
	apps := []db.App{
		{
			Slug:        "direct-app",
			UpstreamURL: "http://localhost:4000",
			Enabled:     true,
			AccessType:  "direct",
		},
	}

	config, err := buildConfig(apps)
	if err != nil {
		t.Fatalf("buildConfig error: %v", err)
	}
	if strings.Contains(config, "/apps/direct-app") {
		t.Errorf("expected no location block for direct app, got:\n%s", config)
	}
}

// TestProxyAppHideHeaders verifies that an enabled proxy app generates
// proxy_hide_header directives for X-Frame-Options and Content-Security-Policy.
func TestProxyAppHideHeaders(t *testing.T) {
	apps := []db.App{
		{
			Slug:        "header-app",
			UpstreamURL: "http://localhost:5000",
			Enabled:     true,
			AccessType:  "proxy",
		},
	}

	config, err := buildConfig(apps)
	if err != nil {
		t.Fatalf("buildConfig error: %v", err)
	}
	if !strings.Contains(config, "proxy_hide_header X-Frame-Options") {
		t.Errorf("expected 'proxy_hide_header X-Frame-Options' in config, got:\n%s", config)
	}
	if !strings.Contains(config, "proxy_hide_header Content-Security-Policy") {
		t.Errorf("expected 'proxy_hide_header Content-Security-Policy' in config, got:\n%s", config)
	}
}

// TestProxyAppSetHeaders verifies that an enabled proxy app generates all four
// proxy_set_header directives.
func TestProxyAppSetHeaders(t *testing.T) {
	apps := []db.App{
		{
			Slug:        "setheader-app",
			UpstreamURL: "http://localhost:5001",
			Enabled:     true,
			AccessType:  "proxy",
		},
	}

	config, err := buildConfig(apps)
	if err != nil {
		t.Fatalf("buildConfig error: %v", err)
	}
	for _, header := range []string{"Host", "X-Real-IP", "X-Forwarded-For", "X-Forwarded-Proto"} {
		if !strings.Contains(config, "proxy_set_header "+header) {
			t.Errorf("expected 'proxy_set_header %s' in config, got:\n%s", header, config)
		}
	}
}

// TestDisabledProxyAppNoLocationBlock verifies that a disabled proxy app does not
// produce a location block even though its AccessType is "proxy".
func TestDisabledProxyAppNoLocationBlock(t *testing.T) {
	apps := []db.App{
		{
			Slug:        "disabled-proxy",
			UpstreamURL: "http://localhost:6000",
			Enabled:     false,
			AccessType:  "proxy",
		},
	}

	config, err := buildConfig(apps)
	if err != nil {
		t.Fatalf("buildConfig error: %v", err)
	}
	if strings.Contains(config, "/apps/disabled-proxy") {
		t.Errorf("expected no location block for disabled proxy app, got:\n%s", config)
	}
}

// TestAtomicWrite verifies that Apply writes the config file atomically:
// the final file exists and the temporary .tmp file does not.
func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "nginx.conf")

	c := NewWithConfig(configPath, nopPID)
	apps := []db.App{
		{
			Slug:        "myapp",
			UpstreamURL: "http://localhost:3000",
			Enabled:     true,
			AccessType:  "proxy",
		},
	}

	if err := c.Apply(context.Background(), apps); err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	// The config file must exist after Apply.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config file %q does not exist after Apply", configPath)
	}

	// The temporary file must not exist after Apply.
	tmpPath := configPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temporary file %q still exists after Apply", tmpPath)
	}
}
