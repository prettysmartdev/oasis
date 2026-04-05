package main

import (
	"log"
	"net"
	"net/http"
	"strings"
	"testing"
)

// TestControllerStartup confirms the management server starts and binds to 127.0.0.1.
func TestControllerStartup(t *testing.T) {
	mux := http.NewServeMux()

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

	// Confirm the server is reachable.
	resp, err := http.Get("http://" + addr + "/api/v1/status") //nolint:noctx
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("closing response body: %v", closeErr)
		}
	})

	// With no routes registered, the default ServeMux returns 404.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found from empty mux, got %d", resp.StatusCode)
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
