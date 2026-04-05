// Package health provides background health checking for registered apps.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// Checker runs periodic health checks against all enabled app upstream URLs.
type Checker struct {
	store    *db.Store
	interval time.Duration
	client   *http.Client
}

// New creates a Checker with the given store and check interval.
func New(store *db.Store, interval time.Duration) *Checker {
	return &Checker{
		store:    store,
		interval: interval,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// Start begins the health check loop. It blocks until ctx is cancelled.
func (c *Checker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runChecks(ctx)
		}
	}
}

// runChecks performs one round of health checks for all enabled apps.
func (c *Checker) runChecks(ctx context.Context) {
	apps, err := c.store.ListApps(ctx)
	if err != nil {
		slog.Warn("health check: failed to list apps", "err", err)
		return
	}

	for _, app := range apps {
		if !app.Enabled {
			continue
		}
		health := c.checkApp(ctx, app.UpstreamURL)
		if err := c.store.SetAppHealth(ctx, app.Slug, health); err != nil {
			slog.Warn("health check: failed to update health", "slug", app.Slug, "err", err)
		}
	}
}

// checkApp makes a HEAD (or GET on failure) request and returns "healthy" or "unreachable".
func (c *Checker) checkApp(ctx context.Context, upstreamURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, upstreamURL, nil)
	if err != nil {
		return "unreachable"
	}
	resp, err := c.client.Do(req)
	if err != nil {
		// Fallback to GET.
		req2, err2 := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
		if err2 != nil {
			return "unreachable"
		}
		resp, err = c.client.Do(req2)
		if err != nil {
			return "unreachable"
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "healthy"
	}
	return "unreachable"
}
