package api

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/prettysmartdev/oasis/internal/controller/db"
	"github.com/prettysmartdev/oasis/internal/controller/nginx"
)

// mockNodeStartable is a TsnetNode that returns a real listener from Start().
type mockNodeStartable struct{}

func (m *mockNodeStartable) IsStarted() bool { return false }
func (m *mockNodeStartable) TailscaleIP() (string, error) {
	return "100.64.0.1", nil
}
func (m *mockNodeStartable) TailscaleDNSName(_ context.Context) (string, error) {
	return "oasis.ts.net", nil
}
func (m *mockNodeStartable) Start(_ context.Context) (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

// newHandlerWithStartableNode creates a Handler with a node that supports Start().
func newHandlerWithStartableNode(t *testing.T) (*Handler, *mockNodeStartable) {
	t.Helper()
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	node := &mockNodeStartable{}
	configurator := nginx.NewWithConfig("", nopPID)
	h := New(store, configurator, node, false)
	return h, node
}

// TestClaudeEnvReturnsNilWhenTokenEmpty verifies claudeEnv() returns nil when no token is set.
func TestClaudeEnvReturnsNilWhenTokenEmpty(t *testing.T) {
	h := newTestHandler(t)
	// claudeOAuthToken defaults to "" in newTestHandler.
	result := h.claudeEnv()
	if result != nil {
		t.Errorf("claudeEnv() with empty token: got %v, want nil", result)
	}
}

// TestClaudeEnvReturnsTokenWhenSet verifies claudeEnv() returns the expected env slice.
func TestClaudeEnvReturnsTokenWhenSet(t *testing.T) {
	h := newTestHandler(t)
	h.claudeOAuthToken = "mytoken"
	result := h.claudeEnv()
	if len(result) != 1 {
		t.Fatalf("claudeEnv() length: got %d, want 1", len(result))
	}
	if result[0] != "CLAUDE_CODE_OAUTH_TOKEN=mytoken" {
		t.Errorf("claudeEnv()[0]: got %q, want %q", result[0], "CLAUDE_CODE_OAUTH_TOKEN=mytoken")
	}
}

// TestSetupStoresClaudeOAuthToken verifies that POST /api/v1/setup stores the claude_oauth_token.
func TestSetupStoresClaudeOAuthToken(t *testing.T) {
	h, _ := newHandlerWithStartableNode(t)
	mux := serveMux(h)

	var ln net.Listener
	h.SetOnSetup(func(l net.Listener) {
		ln = l
	})
	t.Cleanup(func() {
		if ln != nil {
			ln.Close()
		}
	})

	body := `{"tailscaleAuthKey":"key","hostname":"oasis","claude_oauth_token":"mytoken"}`
	rec := doRequest(t, mux, "POST", "/api/v1/setup", body)
	if rec.Code != 200 {
		t.Fatalf("setup: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if h.claudeOAuthToken != "mytoken" {
		t.Errorf("claudeOAuthToken: got %q, want %q", h.claudeOAuthToken, "mytoken")
	}
}

// TestSetupWithoutClaudeTokenLeavesFieldEmpty verifies that omitting claude_oauth_token
// leaves the field as an empty string.
func TestSetupWithoutClaudeTokenLeavesFieldEmpty(t *testing.T) {
	h, _ := newHandlerWithStartableNode(t)
	mux := serveMux(h)

	var ln net.Listener
	h.SetOnSetup(func(l net.Listener) {
		ln = l
	})
	t.Cleanup(func() {
		if ln != nil {
			ln.Close()
		}
	})

	body := `{"tailscaleAuthKey":"key","hostname":"oasis"}`
	rec := doRequest(t, mux, "POST", "/api/v1/setup", body)
	if rec.Code != 200 {
		t.Fatalf("setup: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if h.claudeOAuthToken != "" {
		t.Errorf("claudeOAuthToken: got %q, want empty", h.claudeOAuthToken)
	}
}

// TestSetupResponseDoesNotContainToken verifies that the setup response body never
// leaks the claude_oauth_token value.
func TestSetupResponseDoesNotContainToken(t *testing.T) {
	h, _ := newHandlerWithStartableNode(t)
	mux := serveMux(h)

	var ln net.Listener
	h.SetOnSetup(func(l net.Listener) {
		ln = l
	})
	t.Cleanup(func() {
		if ln != nil {
			ln.Close()
		}
	})

	body := `{"tailscaleAuthKey":"key","hostname":"oasis","claude_oauth_token":"mytoken"}`
	rec := doRequest(t, mux, "POST", "/api/v1/setup", body)
	if rec.Code != 200 {
		t.Fatalf("setup: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "mytoken") {
		t.Errorf("response body must not contain the token; got: %s", rec.Body.String())
	}
}
