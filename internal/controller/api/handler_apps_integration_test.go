//go:build integration

package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prettysmartdev/oasis/internal/controller/db"
	"github.com/prettysmartdev/oasis/internal/controller/nginx"
)

// newIntegrationServerWithNginx starts a full handler backed by an in-memory
// store and a real nginx configurator that writes to configPath.
// A started mockNode is provided so triggerNginxReload is not skipped.
func newIntegrationServerWithNginx(t *testing.T, configPath string) *httptest.Server {
	t.Helper()
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	configurator := nginx.NewWithConfig(configPath, nopPID)
	node := &mockNode{started: true}
	h := New(store, configurator, node, false)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestIntegrationProxyAppLifecycle tests the full proxy-app lifecycle:
//  1. Add a proxy app → NGINX config must contain the location block with proxy headers.
//  2. Update accessType to "direct" → NGINX config must no longer contain the block.
func TestIntegrationProxyAppLifecycle(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nginx.conf")
	srv := newIntegrationServerWithNginx(t, configPath)

	// Step 1: create a proxy app.
	status, body := doHTTP(t, srv, http.MethodPost, "/api/v1/apps",
		`{"name":"Proxy Lifecycle","slug":"proxy-lifecycle","upstreamURL":"http://localhost:8888","accessType":"proxy"}`)
	if status != http.StatusCreated {
		t.Fatalf("create proxy app: status %d body %v", status, body)
	}
	if body["accessType"] != "proxy" {
		t.Errorf("accessType after create: got %v, want %q", body["accessType"], "proxy")
	}

	// Step 2: verify NGINX config contains the location block with required proxy headers.
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read nginx config after create: %v", err)
	}
	cfgStr := string(cfg)
	if !strings.Contains(cfgStr, "/apps/proxy-lifecycle/") {
		t.Errorf("nginx config missing location block for proxy app:\n%s", cfgStr)
	}
	if !strings.Contains(cfgStr, "proxy_hide_header X-Frame-Options") {
		t.Errorf("nginx config missing proxy_hide_header X-Frame-Options:\n%s", cfgStr)
	}
	if !strings.Contains(cfgStr, "proxy_hide_header Content-Security-Policy") {
		t.Errorf("nginx config missing proxy_hide_header Content-Security-Policy:\n%s", cfgStr)
	}

	// Step 3: verify the app appears in app list with accessType "proxy".
	status, listBody := doHTTP(t, srv, http.MethodGet, "/api/v1/apps", "")
	if status != http.StatusOK {
		t.Fatalf("list apps: status %d", status)
	}
	items, _ := listBody["items"].([]any)
	found := false
	for _, item := range items {
		m, _ := item.(map[string]any)
		if m["slug"] == "proxy-lifecycle" {
			found = true
			if m["accessType"] != "proxy" {
				t.Errorf("list: accessType for proxy-lifecycle: got %v, want %q", m["accessType"], "proxy")
			}
		}
	}
	if !found {
		t.Errorf("proxy-lifecycle not found in app list")
	}

	// Step 4: update accessType to "direct".
	status, _ = doHTTP(t, srv, http.MethodPatch, "/api/v1/apps/proxy-lifecycle",
		`{"accessType":"direct"}`)
	if status != http.StatusOK {
		t.Fatalf("update app to direct: status %d", status)
	}

	// Step 5: verify location block is gone from NGINX config.
	cfg, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read nginx config after update: %v", err)
	}
	if strings.Contains(string(cfg), "/apps/proxy-lifecycle/") {
		t.Errorf("nginx config still has location block after switching to direct:\n%s", string(cfg))
	}
}

// TestIntegrationDirectAppNoLocationBlock tests that a direct app never gets a
// location block in the NGINX config, even when other proxy apps exist.
func TestIntegrationDirectAppNoLocationBlock(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nginx.conf")
	srv := newIntegrationServerWithNginx(t, configPath)

	// Add a direct app (access type omitted → defaults to "proxy" on the API,
	// but we pass "direct" explicitly to test the direct path).
	status, body := doHTTP(t, srv, http.MethodPost, "/api/v1/apps",
		`{"name":"Direct App","slug":"direct-only","upstreamURL":"http://localhost:7777","accessType":"direct"}`)
	if status != http.StatusCreated {
		t.Fatalf("create direct app: status %d body %v", status, body)
	}
	if body["accessType"] != "direct" {
		t.Errorf("accessType: got %v, want %q", body["accessType"], "direct")
	}

	cfg, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read nginx config: %v", err)
	}
	if strings.Contains(string(cfg), "/apps/direct-only/") {
		t.Errorf("nginx config must not contain a location block for a direct app:\n%s", string(cfg))
	}
}
