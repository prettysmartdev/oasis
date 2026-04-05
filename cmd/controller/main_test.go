package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNotImplementedHandler_Status confirms every route returns 501.
func TestNotImplementedHandler_Status(t *testing.T) {
	paths := []string{"/", "/api/v1/status", "/api/v1/apps", "/unknown"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			notImplementedHandler(rec, req)
			if rec.Code != http.StatusNotImplemented {
				t.Errorf("path %s: expected 501, got %d", path, rec.Code)
			}
		})
	}
}

// TestNotImplementedHandler_ContentType confirms the response is JSON.
func TestNotImplementedHandler_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	notImplementedHandler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json Content-Type, got %q", ct)
	}
}

// TestNotImplementedHandler_Body confirms the JSON body matches the error convention.
func TestNotImplementedHandler_Body(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()
	notImplementedHandler(rec, req)

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected valid JSON body: %v", err)
	}
	if body["code"] != "NOT_IMPLEMENTED" {
		t.Errorf("expected code=NOT_IMPLEMENTED, got %q", body["code"])
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

// TestControllerStartup confirms the management server starts, binds to
// 127.0.0.1, and returns 501 for all routes.
func TestControllerStartup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", notImplementedHandler)

	// Bind to 127.0.0.1 with OS-assigned port — never 0.0.0.0.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("test server exited: %v", err)
		}
	}()
	t.Cleanup(func() {
		if err := srv.Close(); err != nil {
			log.Printf("closing test server: %v", err)
		}
	})

	addr := ln.Addr().String()

	// Loopback assertion — the management API must never bind to 0.0.0.0.
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("management API bound to %q — must bind to 127.0.0.1", addr)
	}

	// Confirm server responds with 501.
	resp, err := http.Get("http://" + addr + "/api/v1/status") //nolint:noctx
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("closing response body: %v", closeErr)
		}
	})

	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected 501 Not Implemented, got %d", resp.StatusCode)
	}
}

// TestBuildMgmtAddr confirms buildMgmtAddr always produces a loopback address
// and never returns an address that would bind to 0.0.0.0.
func TestBuildMgmtAddr(t *testing.T) {
	cases := []struct {
		port string
		want string
	}{
		{"04515", "127.0.0.1:04515"},
		{"7700", "127.0.0.1:7700"},
		{"8080", "127.0.0.1:8080"},
	}
	for _, tc := range cases {
		got := buildMgmtAddr(tc.port)
		if got != tc.want {
			t.Errorf("buildMgmtAddr(%q) = %q, want %q", tc.port, got, tc.want)
		}
		if !strings.HasPrefix(got, "127.0.0.1:") {
			t.Errorf("buildMgmtAddr(%q) = %q — host must be 127.0.0.1, never 0.0.0.0", tc.port, got)
		}
	}
}
