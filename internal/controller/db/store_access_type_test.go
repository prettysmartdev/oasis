package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// ptr is a helper that returns a pointer to the given string value.
func ptr(s string) *string { return &s }

// newTestAppWithAccessType builds a test App with the given slug and accessType.
func newTestAppWithAccessType(slug, accessType string) App {
	now := time.Now().UTC().Truncate(time.Second)
	return App{
		ID:          "test-id-" + slug,
		Name:        "Test App " + slug,
		Slug:        slug,
		UpstreamURL: "http://localhost:3000",
		DisplayName: "Display " + slug,
		Description: "A test app",
		Icon:        "icon.png",
		Tags:        []string{},
		Enabled:     true,
		Health:      "unknown",
		AccessType:  accessType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TestCreateAppWithProxyAccessType verifies that an app created with AccessType="proxy"
// is persisted and retrieved with the same value.
func TestCreateAppWithProxyAccessType(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	app := newTestAppWithAccessType("proxy-app", "proxy")
	if err := s.CreateApp(ctx, app); err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}

	got, err := s.GetApp(ctx, app.Slug)
	if err != nil {
		t.Fatalf("GetApp error: %v", err)
	}
	if got.AccessType != "proxy" {
		t.Errorf("AccessType: got %q, want %q", got.AccessType, "proxy")
	}
}

// TestCreateAppAccessTypeDirectRoundTrip verifies that an app created with AccessType="direct"
// survives a round-trip through the database.
func TestCreateAppAccessTypeDirectRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	app := newTestAppWithAccessType("direct-app", "direct")
	if err := s.CreateApp(ctx, app); err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}

	got, err := s.GetApp(ctx, app.Slug)
	if err != nil {
		t.Fatalf("GetApp error: %v", err)
	}
	if got.AccessType != "direct" {
		t.Errorf("AccessType: got %q, want %q", got.AccessType, "direct")
	}
}

// TestUpdateAppAccessType verifies that UpdateApp with an AccessType patch changes the value
// both in the returned struct and in subsequent GetApp calls.
func TestUpdateAppAccessType(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	app := newTestAppWithAccessType("update-access", "direct")
	if err := s.CreateApp(ctx, app); err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}

	updated, err := s.UpdateApp(ctx, app.Slug, AppPatch{AccessType: ptr("proxy")})
	if err != nil {
		t.Fatalf("UpdateApp error: %v", err)
	}
	if updated.AccessType != "proxy" {
		t.Errorf("updated AccessType: got %q, want %q", updated.AccessType, "proxy")
	}

	// Confirm the new value is persisted.
	got, err := s.GetApp(ctx, app.Slug)
	if err != nil {
		t.Fatalf("GetApp after update error: %v", err)
	}
	if got.AccessType != "proxy" {
		t.Errorf("persisted AccessType: got %q, want %q", got.AccessType, "proxy")
	}
}

// TestListAppsAccessType verifies that ListApps returns the correct AccessType for each app.
func TestListAppsAccessType(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	directApp := newTestAppWithAccessType("list-direct", "direct")
	proxyApp := newTestAppWithAccessType("list-proxy", "proxy")

	if err := s.CreateApp(ctx, directApp); err != nil {
		t.Fatalf("CreateApp direct: %v", err)
	}
	if err := s.CreateApp(ctx, proxyApp); err != nil {
		t.Fatalf("CreateApp proxy: %v", err)
	}

	apps, err := s.ListApps(ctx)
	if err != nil {
		t.Fatalf("ListApps error: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("ListApps count: got %d, want 2", len(apps))
	}

	bySlug := make(map[string]App, len(apps))
	for _, a := range apps {
		bySlug[a.Slug] = a
	}

	if bySlug["list-direct"].AccessType != "direct" {
		t.Errorf("list-direct AccessType: got %q, want %q", bySlug["list-direct"].AccessType, "direct")
	}
	if bySlug["list-proxy"].AccessType != "proxy" {
		t.Errorf("list-proxy AccessType: got %q, want %q", bySlug["list-proxy"].AccessType, "proxy")
	}
}

// TestMigration3OnVersion2DB verifies that opening a version-2 database (without access_type)
// runs migration 3, adding the column and allowing new apps to store AccessType correctly.
func TestMigration3OnVersion2DB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v2test.db")

	// Build a schema that matches the state after migration 2 (no access_type column).
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}

	// Create tables in their migration-2 state.
	_, err = rawDB.Exec(`
CREATE TABLE apps (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    upstream_url TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    tags         TEXT NOT NULL DEFAULT '[]',
    enabled      INTEGER NOT NULL DEFAULT 1,
    health       TEXT NOT NULL DEFAULT 'unknown',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE TABLE settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    tailscale_hostname TEXT NOT NULL DEFAULT 'oasis',
    mgmt_port          INTEGER NOT NULL DEFAULT 4515,
    theme              TEXT NOT NULL DEFAULT 'system'
);
INSERT OR IGNORE INTO settings (id) VALUES (1);
CREATE TABLE agents (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',
    prompt      TEXT NOT NULL,
    trigger     TEXT NOT NULL,
    schedule    TEXT NOT NULL DEFAULT '',
    output_fmt  TEXT NOT NULL DEFAULT 'markdown',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE TABLE agent_runs (
    id          TEXT PRIMARY KEY,
    agent_id    TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    trigger_src TEXT NOT NULL,
    status      TEXT NOT NULL,
    output      TEXT NOT NULL DEFAULT '',
    started_at  TEXT NOT NULL,
    finished_at TEXT
);`)
	if err != nil {
		rawDB.Close()
		t.Fatalf("create v2 schema: %v", err)
	}

	// Set user_version = 2 so the migrator knows to apply only migration 3.
	if _, err := rawDB.Exec("PRAGMA user_version = 2"); err != nil {
		rawDB.Close()
		t.Fatalf("set user_version = 2: %v", err)
	}
	rawDB.Close()

	// Open the store — this should trigger migration 3.
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() on v2 db failed (migration 3 error?): %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Verify that we can now create an app with an AccessType.
	ctx := context.Background()
	app := newTestAppWithAccessType("migrated-app", "proxy")
	if err := store.CreateApp(ctx, app); err != nil {
		t.Fatalf("CreateApp after migration 3: %v", err)
	}

	got, err := store.GetApp(ctx, app.Slug)
	if err != nil {
		t.Fatalf("GetApp after migration 3: %v", err)
	}
	if got.AccessType != "proxy" {
		t.Errorf("AccessType after migration 3: got %q, want %q", got.AccessType, "proxy")
	}
}
