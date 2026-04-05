package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prettysmartdev/oasis/internal/controller/db"
	"github.com/prettysmartdev/oasis/internal/controller/nginx"
)

// mockNode implements TsnetNode for use in tests.
type mockNode struct {
	started bool
}

func (m *mockNode) IsStarted() bool { return m.started }
func (m *mockNode) TailscaleIP() (string, error) {
	if !m.started {
		return "", fmt.Errorf("not started")
	}
	return "100.64.0.1", nil
}
func (m *mockNode) Start(_ context.Context) (net.Listener, error) {
	return nil, fmt.Errorf("mock: Start not implemented")
}

// nopPID returns an error so SIGHUP is always skipped in handler tests.
func nopPID() (int, error) {
	return 0, fmt.Errorf("no nginx in test")
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	configurator := nginx.NewWithConfig("", nopPID)
	return New(store, configurator, nil, false)
}

func newReadOnlyHandler(t *testing.T) *Handler {
	t.Helper()
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	configurator := nginx.NewWithConfig("", nopPID)
	return New(store, configurator, nil, true)
}

func serveMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func doRequest(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// TestGetStatus verifies that GET /api/v1/status returns 200 with a version field.
func TestGetStatus(t *testing.T) {
	h := newTestHandler(t)
	h.SetVersion("test-version")
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/api/v1/status", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := resp["version"]; !ok {
		t.Errorf("response missing 'version' field: %v", resp)
	}
}

// TestListAppsEmpty verifies GET /api/v1/apps returns 200 with empty items on a fresh db.
func TestListAppsEmpty(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/api/v1/apps", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := resp["items"]; !ok {
		t.Errorf("response missing 'items' field: %v", resp)
	}
	if _, ok := resp["total"]; !ok {
		t.Errorf("response missing 'total' field: %v", resp)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("'items' is not a slice: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// TestCreateAppHappyPath verifies POST /api/v1/apps returns 201 on a valid request.
func TestCreateAppHappyPath(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"My App","slug":"my-app","upstreamURL":"http://localhost:8080"}`
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", body)
	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

// TestCreateAppInvalidSlug verifies POST /api/v1/apps returns 400 for an invalid slug.
func TestCreateAppInvalidSlug(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"My App","slug":"INVALID SLUG!","upstreamURL":"http://localhost:8080"}`
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateAppMissingName verifies POST /api/v1/apps returns 400 when name is absent.
func TestCreateAppMissingName(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"slug":"my-app","upstreamURL":"http://localhost:8080"}`
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateAppInvalidUpstreamURL verifies that non-HTTP/HTTPS and malformed upstream URLs
// are rejected with 400.
func TestCreateAppInvalidUpstreamURL(t *testing.T) {
	cases := []struct {
		name        string
		upstreamURL string
	}{
		{"empty", ""},
		{"not a url", "not a url"},
		{"ftp scheme", "ftp://localhost:21"},
		{"missing host", "http://"},
		{"no scheme", "localhost:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(t)
			mux := serveMux(h)
			body := fmt.Sprintf(`{"name":"App","slug":"app","upstreamURL":%q}`, tc.upstreamURL)
			rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("upstreamURL=%q: got %d, want 400; body: %s", tc.upstreamURL, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestCreateAppDuplicateSlug verifies POST /api/v1/apps returns 409 on a slug conflict.
func TestCreateAppDuplicateSlug(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"App One","slug":"dup-slug","upstreamURL":"http://localhost:8080"}`
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create: got %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	rec2 := doRequest(t, mux, http.MethodPost, "/api/v1/apps", body)
	if rec2.Code != http.StatusConflict {
		t.Errorf("duplicate create: got %d, want 409; body: %s", rec2.Code, rec2.Body.String())
	}
}

// TestGetAppFound verifies GET /api/v1/apps/{slug} returns 200 for an existing app.
func TestGetAppFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	createBody := `{"name":"Test App","slug":"test-app","upstreamURL":"http://localhost:9000"}`
	if rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", createBody); rec.Code != http.StatusCreated {
		t.Fatalf("create app: got %d", rec.Code)
	}

	rec := doRequest(t, mux, http.MethodGet, "/api/v1/apps/test-app", "")
	if rec.Code != http.StatusOK {
		t.Errorf("get app: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// TestGetAppNotFound verifies GET /api/v1/apps/{slug} returns 404 for a missing slug.
func TestGetAppNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/api/v1/apps/not-here", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("get missing app: got %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateAppFound verifies PATCH /api/v1/apps/{slug} returns 200 for an existing app.
func TestUpdateAppFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	createBody := `{"name":"Patch Me","slug":"patch-app","upstreamURL":"http://localhost:9001"}`
	if rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", createBody); rec.Code != http.StatusCreated {
		t.Fatalf("create app: got %d", rec.Code)
	}

	patchBody := `{"name":"Patched Name"}`
	rec := doRequest(t, mux, http.MethodPatch, "/api/v1/apps/patch-app", patchBody)
	if rec.Code != http.StatusOK {
		t.Errorf("patch app: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateAppNotFound verifies PATCH /api/v1/apps/{slug} returns 404 for a missing slug.
func TestUpdateAppNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodPatch, "/api/v1/apps/no-such-app", `{"name":"X"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("patch missing app: got %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

// TestDeleteAppFound verifies DELETE /api/v1/apps/{slug} returns 204 for an existing app.
func TestDeleteAppFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	createBody := `{"name":"Delete Me","slug":"delete-app","upstreamURL":"http://localhost:9002"}`
	if rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", createBody); rec.Code != http.StatusCreated {
		t.Fatalf("create app: got %d", rec.Code)
	}

	rec := doRequest(t, mux, http.MethodDelete, "/api/v1/apps/delete-app", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete app: got %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
}

// TestDeleteAppNotFound verifies DELETE /api/v1/apps/{slug} returns 404 for a missing slug.
func TestDeleteAppNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodDelete, "/api/v1/apps/ghost", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete missing app: got %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

// TestEnableApp verifies POST /api/v1/apps/{slug}/enable returns 200 for an existing app.
func TestEnableApp(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	createBody := `{"name":"Toggle App","slug":"toggle-app","upstreamURL":"http://localhost:9003"}`
	if rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", createBody); rec.Code != http.StatusCreated {
		t.Fatalf("create app: got %d", rec.Code)
	}

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps/toggle-app/enable", "")
	if rec.Code != http.StatusOK {
		t.Errorf("enable app: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// TestEnableAppNotFound verifies POST /api/v1/apps/{slug}/enable returns 404 for a missing slug.
func TestEnableAppNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps/ghost/enable", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("enable missing app: got %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

// TestDisableApp verifies POST /api/v1/apps/{slug}/disable returns 200 for an existing app.
func TestDisableApp(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	createBody := `{"name":"Disable App","slug":"disable-app","upstreamURL":"http://localhost:9004"}`
	if rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", createBody); rec.Code != http.StatusCreated {
		t.Fatalf("create app: got %d", rec.Code)
	}

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps/disable-app/disable", "")
	if rec.Code != http.StatusOK {
		t.Errorf("disable app: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// TestDisableAppNotFound verifies POST /api/v1/apps/{slug}/disable returns 404 for a missing slug.
func TestDisableAppNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps/ghost/disable", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("disable missing app: got %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

// TestGetSettingsResponse verifies GET /api/v1/settings returns 200 and the JSON body
// contains no "tailscaleAuthKey" field.
func TestGetSettingsResponse(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/api/v1/settings", "")
	if rec.Code != http.StatusOK {
		t.Errorf("get settings: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, "tailscaleAuthKey") {
		t.Errorf("response must not contain 'tailscaleAuthKey'; got: %s", body)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal settings response: %v", err)
	}
	if _, ok := resp["tailscaleHostname"]; !ok {
		t.Errorf("expected 'tailscaleHostname' field in response: %v", resp)
	}
}

// TestUpdateSettings verifies PATCH /api/v1/settings returns 200.
func TestUpdateSettings(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, http.MethodPatch, "/api/v1/settings", `{"theme":"dark"}`)
	if rec.Code != http.StatusOK {
		t.Errorf("patch settings: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// TestSetupNilNode verifies POST /api/v1/setup returns 503 when node is nil.
func TestSetupNilNode(t *testing.T) {
	h := newTestHandler(t) // node is nil
	mux := serveMux(h)

	body := `{"tailscaleAuthKey":"tskey-abc","hostname":"oasis"}`
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/setup", body)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("setup nil node: got %d, want 503; body: %s", rec.Code, rec.Body.String())
	}
}

// TestSetupAlreadyStarted verifies POST /api/v1/setup returns 409 when the node is already started.
func TestSetupAlreadyStarted(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	node := &mockNode{started: true}
	configurator := nginx.NewWithConfig("", nopPID)
	h := New(store, configurator, node, false)
	mux := serveMux(h)

	body := `{"tailscaleAuthKey":"tskey-abc","hostname":"oasis"}`
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/setup", body)
	if rec.Code != http.StatusConflict {
		t.Errorf("setup already started: got %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
}

// TestReadOnlyHandlerRejectsWriteMethods verifies that write endpoints return 405
// when a read-only handler does not register them (Go 1.22+ mux behaviour).
func TestReadOnlyHandlerRejectsWriteMethods(t *testing.T) {
	h := newReadOnlyHandler(t)
	mux := serveMux(h)

	// GET /api/v1/apps is registered; POST to the same path → 405.
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/apps", `{"name":"x","slug":"x","upstreamURL":"http://localhost"}`)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("read-only POST /api/v1/apps: got %d, want 405; body: %s", rec.Code, rec.Body.String())
	}
}
