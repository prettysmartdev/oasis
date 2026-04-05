package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// TestCheckAppHealthy verifies that a server returning 200 is reported as "healthy".
func TestCheckAppHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	c := New(store, time.Minute)
	got := c.checkApp(context.Background(), srv.URL)
	if got != "healthy" {
		t.Errorf("checkApp healthy server: got %q, want %q", got, "healthy")
	}
}

// TestCheckAppUnreachable verifies that an unreachable URL is reported as "unreachable".
func TestCheckAppUnreachable(t *testing.T) {
	// Use a server we immediately close so the port is unavailable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // close before the check

	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	c := New(store, time.Minute)
	got := c.checkApp(context.Background(), url)
	if got != "unreachable" {
		t.Errorf("checkApp closed server: got %q, want %q", got, "unreachable")
	}
}

// TestCheckerContextCancel verifies that Start returns promptly when the context is cancelled.
func TestCheckerContextCancel(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	c := New(store, time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Start returned promptly after context cancellation.
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after context cancellation within 2s")
	}
}

// TestSetAppHealthCalled verifies that after one tick the checker writes the
// health status back to the store for an enabled app.
func TestSetAppHealthCalled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	app := db.App{
		ID:          "health-test-id",
		Name:        "Health App",
		Slug:        "health-app",
		UpstreamURL: srv.URL,
		Enabled:     true,
		Health:      "unknown",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.CreateApp(ctx, app); err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}

	// Use a very short interval so at least one check runs quickly.
	c := New(store, 5*time.Millisecond)
	checkCtx, cancel := context.WithCancel(ctx)

	go c.Start(checkCtx)

	// Wait up to 1 second for the health to be updated.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, err := store.GetApp(ctx, app.Slug)
		if err != nil {
			t.Fatalf("GetApp error: %v", err)
		}
		if got.Health == "healthy" {
			cancel()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Error("app health was not updated to 'healthy' within 1 second")
}
