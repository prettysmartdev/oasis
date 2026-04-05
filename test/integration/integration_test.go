//go:build integration

// Package integration_test contains end-to-end tests that run against a live
// oasis controller instance. Start the controller before running these tests:
//
//	go test -tags integration ./test/integration/...
//
// Two lifecycle scenarios cannot be automated here and must be verified manually:
//
//  1. Database persistence across restart: create an app, docker stop/start the
//     container, then GET /api/v1/apps and confirm the app is still present.
//
//  2. NGINX config contents after disable: POST /api/v1/apps/:slug/disable, then
//     exec into the container and confirm /etc/nginx/nginx.conf no longer contains
//     the slug (requires shell access to the running container).
package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

const baseURL = "http://127.0.0.1:04515"

// doJSON sends an HTTP request with an optional JSON body and decodes the
// response body into dest (if non-nil).
func doJSON(t *testing.T, method, path string, body any, dest any) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Oasis-CLI-Version", "integration-test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp
}

// TestStatusEndpoint verifies that the management API is reachable, returns 200,
// and reports nginxStatus "running" — confirming NGINX is up inside the container.
func TestStatusEndpoint(t *testing.T) {
	var status map[string]any
	resp := doJSON(t, http.MethodGet, "/api/v1/status", nil, &status)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/status: got %d, want 200", resp.StatusCode)
	}
	if got, _ := status["nginxStatus"].(string); got != "running" {
		t.Errorf("nginxStatus: got %q, want %q", got, "running")
	}
}

// TestAppLifecycle exercises the full create → get → disable → delete lifecycle.
func TestAppLifecycle(t *testing.T) {
	slug := fmt.Sprintf("integration-test-%d", uniqueSuffix())

	// Ensure a clean state by attempting a pre-emptive delete (ignore errors).
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/v1/apps/"+slug, nil)
	http.DefaultClient.Do(req) //nolint:errcheck

	// Create.
	createBody := map[string]any{
		"name":        "Integration Test App",
		"slug":        slug,
		"upstreamURL": "http://localhost:19999",
		"enabled":     true,
	}
	var created map[string]any
	createResp := doJSON(t, http.MethodPost, "/api/v1/apps", createBody, &created)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create app: got %d, want 201", createResp.StatusCode)
	}
	if created["slug"] != slug {
		t.Errorf("created slug: got %v, want %q", created["slug"], slug)
	}

	// Get.
	var got map[string]any
	getResp := doJSON(t, http.MethodGet, "/api/v1/apps/"+slug, nil, &got)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get app: got %d, want 200", getResp.StatusCode)
	}
	if got["slug"] != slug {
		t.Errorf("get slug: got %v, want %q", got["slug"], slug)
	}

	// Disable.
	disableResp := doJSON(t, http.MethodPost, "/api/v1/apps/"+slug+"/disable", nil, nil)
	if disableResp.StatusCode != http.StatusOK {
		t.Fatalf("disable app: got %d, want 200", disableResp.StatusCode)
	}

	// Delete.
	deleteResp := doJSON(t, http.MethodDelete, "/api/v1/apps/"+slug, nil, nil)
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete app: got %d, want 204", deleteResp.StatusCode)
	}

	// Confirm deletion — GET must return 404.
	getAfterDelete := doJSON(t, http.MethodGet, "/api/v1/apps/"+slug, nil, nil)
	if getAfterDelete.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete: got %d, want 404", getAfterDelete.StatusCode)
	}
}

// uniqueSuffix returns a simple counter value so parallel test runs use distinct slugs.
var counter int

func uniqueSuffix() int {
	counter++
	return counter
}
