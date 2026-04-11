package api

import (
	"context"
	"net"
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

// TestSetupSucceeds verifies that POST /api/v1/setup with valid fields returns 200.
func TestSetupSucceeds(t *testing.T) {
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
}
