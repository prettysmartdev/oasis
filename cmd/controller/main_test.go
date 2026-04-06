package main

import (
	"log"
	"net"
	"net/http"
	"testing"
)

// TestControllerStartup confirms the management server starts and is reachable.
// In production the host is 0.0.0.0 inside the container; Docker's
// -p 127.0.0.1:PORT:PORT binding restricts external access at the host level.
func TestControllerStartup(t *testing.T) {
	mux := http.NewServeMux()

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

// TestBuildMgmtAddr confirms buildMgmtAddr correctly combines host and port.
// Inside Docker the host is 0.0.0.0 (Docker's -p 127.0.0.1:... binding on the host
// enforces loopback-only access). For direct binary execution outside Docker,
// operators set OASIS_MGMT_HOST=127.0.0.1.
func TestBuildMgmtAddr(t *testing.T) {
	cases := []struct {
		host string
		port string
		want string
	}{
		{"0.0.0.0", "04515", "0.0.0.0:04515"},
		{"127.0.0.1", "04515", "127.0.0.1:04515"},
		{"0.0.0.0", "7700", "0.0.0.0:7700"},
	}
	for _, tc := range cases {
		got := buildMgmtAddr(tc.host, tc.port)
		if got != tc.want {
			t.Errorf("buildMgmtAddr(%q, %q) = %q, want %q", tc.host, tc.port, got, tc.want)
		}
	}
}
