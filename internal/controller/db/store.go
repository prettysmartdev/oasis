// Package db provides SQLite persistence for the oasis app registry and settings.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	// modernc.org/sqlite is a pure-Go SQLite driver (CGO_ENABLED=0 compatible).
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// App represents a registered application in the oasis registry.
type App struct {
	ID          string
	Name        string
	Slug        string
	UpstreamURL string
	DisplayName string
	Description string
	Icon        string
	Tags        []string
	Enabled     bool
	Health      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AppPatch carries optional fields for a partial App update.
// Only non-nil fields are applied.
type AppPatch struct {
	Name        *string
	UpstreamURL *string
	DisplayName *string
	Description *string
	Icon        *string
	Tags        *[]string
	Enabled     *bool
}

// Settings holds the single-row global controller settings.
type Settings struct {
	TailscaleHostname string
	MgmtPort          int
	Theme             string
}

// SettingsPatch carries optional fields for a partial Settings update.
type SettingsPatch struct {
	TailscaleHostname *string
	MgmtPort          *int
	Theme             *string
}

// Store manages the SQLite database connection and applies schema migrations.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at the given path.
// Use an empty string or ":memory:" for an in-memory database (tests).
// Schema migrations are applied automatically on open.
func New(path string) (*Store, error) {
	if path == "" {
		path = ":memory:"
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite performs best with a single writer connection.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate applies schema migrations gated by PRAGMA user_version.
func (s *Store) migrate(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version >= 1 {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS apps (
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

CREATE TABLE IF NOT EXISTS settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    tailscale_hostname TEXT NOT NULL DEFAULT 'oasis',
    mgmt_port          INTEGER NOT NULL DEFAULT 4515,
    theme              TEXT NOT NULL DEFAULT 'system'
);

INSERT OR IGNORE INTO settings (id) VALUES (1);

PRAGMA user_version = 1;
`)
	return err
}

// CreateApp inserts a new app record.
func (s *Store) CreateApp(ctx context.Context, app App) error {
	tags, err := json.Marshal(app.Tags)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO apps (id, name, slug, upstream_url, display_name, description, icon, tags, enabled, health, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.Name, app.Slug, app.UpstreamURL,
		app.DisplayName, app.Description, app.Icon,
		string(tags),
		boolToInt(app.Enabled), app.Health,
		app.CreatedAt.UTC().Format(time.RFC3339),
		app.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetApp retrieves an app by slug. Returns ErrNotFound if it does not exist.
func (s *Store) GetApp(ctx context.Context, slug string) (App, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, upstream_url, display_name, description, icon, tags, enabled, health, created_at, updated_at
		 FROM apps WHERE slug = ?`, slug)
	return scanApp(row)
}

// ListApps returns all apps ordered by creation time (oldest first).
func (s *Store) ListApps(ctx context.Context) ([]App, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, slug, upstream_url, display_name, description, icon, tags, enabled, health, created_at, updated_at
		 FROM apps ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []App
	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

// UpdateApp applies a partial update to the app identified by slug.
// Returns the updated App or ErrNotFound if the slug does not exist.
func (s *Store) UpdateApp(ctx context.Context, slug string, patch AppPatch) (App, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return App{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var app App
	row := tx.QueryRowContext(ctx,
		`SELECT id, name, slug, upstream_url, display_name, description, icon, tags, enabled, health, created_at, updated_at
		 FROM apps WHERE slug = ?`, slug)
	app, err = scanApp(row)
	if err != nil {
		return App{}, err
	}

	if patch.Name != nil {
		app.Name = *patch.Name
	}
	if patch.UpstreamURL != nil {
		app.UpstreamURL = *patch.UpstreamURL
	}
	if patch.DisplayName != nil {
		app.DisplayName = *patch.DisplayName
	}
	if patch.Description != nil {
		app.Description = *patch.Description
	}
	if patch.Icon != nil {
		app.Icon = *patch.Icon
	}
	if patch.Tags != nil {
		app.Tags = *patch.Tags
	}
	if patch.Enabled != nil {
		app.Enabled = *patch.Enabled
	}

	app.UpdatedAt = time.Now().UTC()
	tags, err := json.Marshal(app.Tags)
	if err != nil {
		return App{}, err
	}

	_, err = tx.ExecContext(ctx, `
UPDATE apps SET name=?, upstream_url=?, display_name=?, description=?, icon=?, tags=?, enabled=?, updated_at=?
WHERE slug=?`,
		app.Name, app.UpstreamURL, app.DisplayName, app.Description,
		app.Icon, string(tags), boolToInt(app.Enabled),
		app.UpdatedAt.Format(time.RFC3339), slug,
	)
	if err != nil {
		return App{}, err
	}

	return app, tx.Commit()
}

// DeleteApp removes the app with the given slug.
// Returns ErrNotFound if the slug does not exist.
func (s *Store) DeleteApp(ctx context.Context, slug string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM apps WHERE slug = ?`, slug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAppHealth updates the health field for the given app slug.
func (s *Store) SetAppHealth(ctx context.Context, slug, health string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE apps SET health = ?, updated_at = ? WHERE slug = ?`,
		health, time.Now().UTC().Format(time.RFC3339), slug,
	)
	return err
}

// GetSettings returns the global settings row.
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var st Settings
	err := s.db.QueryRowContext(ctx,
		`SELECT tailscale_hostname, mgmt_port, theme FROM settings WHERE id = 1`,
	).Scan(&st.TailscaleHostname, &st.MgmtPort, &st.Theme)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{TailscaleHostname: "oasis", MgmtPort: 4515, Theme: "system"}, nil
	}
	return st, err
}

// UpdateSettings applies a partial update to the global settings.
func (s *Store) UpdateSettings(ctx context.Context, patch SettingsPatch) (Settings, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Settings{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var st Settings
	err = tx.QueryRowContext(ctx,
		`SELECT tailscale_hostname, mgmt_port, theme FROM settings WHERE id = 1`,
	).Scan(&st.TailscaleHostname, &st.MgmtPort, &st.Theme)
	if err != nil {
		return Settings{}, err
	}

	if patch.TailscaleHostname != nil {
		st.TailscaleHostname = *patch.TailscaleHostname
	}
	if patch.MgmtPort != nil {
		st.MgmtPort = *patch.MgmtPort
	}
	if patch.Theme != nil {
		st.Theme = *patch.Theme
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE settings SET tailscale_hostname=?, mgmt_port=?, theme=? WHERE id=1`,
		st.TailscaleHostname, st.MgmtPort, st.Theme,
	)
	if err != nil {
		return Settings{}, err
	}
	return st, tx.Commit()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanApp(s scanner) (App, error) {
	var (
		app      App
		tagsJSON string
		enabledI int
		createdS string
		updatedS string
	)
	err := s.Scan(
		&app.ID, &app.Name, &app.Slug, &app.UpstreamURL,
		&app.DisplayName, &app.Description, &app.Icon,
		&tagsJSON, &enabledI, &app.Health,
		&createdS, &updatedS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return App{}, ErrNotFound
	}
	if err != nil {
		return App{}, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &app.Tags); err != nil {
		app.Tags = []string{}
	}
	app.Enabled = enabledI != 0
	app.CreatedAt, _ = time.Parse(time.RFC3339, createdS)
	app.UpdatedAt, _ = time.Parse(time.RFC3339, updatedS)
	return app, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
