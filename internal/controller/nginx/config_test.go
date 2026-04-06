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

// TestApplyEnabledApp verifies that an enabled app produces a proxy_pass location block.
func TestApplyEnabledApp(t *testing.T) {
	apps := []db.App{
		{
			Slug:        "myapp",
			UpstreamURL: "http://localhost:3000",
			Enabled:     true,
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
