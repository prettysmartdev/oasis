package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(':memory:') error: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Logf("close store: %v", err)
		}
	})
	return s
}

func newTestApp(slug string) App {
	now := time.Now().UTC().Truncate(time.Second)
	return App{
		ID:          "test-id-" + slug,
		Name:        "Test App " + slug,
		Slug:        slug,
		UpstreamURL: "http://localhost:3000",
		DisplayName: "Display " + slug,
		Description: "A test app",
		Icon:        "icon.png",
		Tags:        []string{"tag1", "tag2"},
		Enabled:     true,
		Health:      "unknown",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TestCreateGetRoundTrip verifies that CreateApp followed by GetApp preserves all fields.
func TestCreateGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	app := newTestApp("roundtrip")
	if err := s.CreateApp(ctx, app); err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}

	got, err := s.GetApp(ctx, app.Slug)
	if err != nil {
		t.Fatalf("GetApp error: %v", err)
	}

	if got.ID != app.ID {
		t.Errorf("ID: got %q, want %q", got.ID, app.ID)
	}
	if got.Name != app.Name {
		t.Errorf("Name: got %q, want %q", got.Name, app.Name)
	}
	if got.Slug != app.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, app.Slug)
	}
	if got.UpstreamURL != app.UpstreamURL {
		t.Errorf("UpstreamURL: got %q, want %q", got.UpstreamURL, app.UpstreamURL)
	}
	if got.DisplayName != app.DisplayName {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, app.DisplayName)
	}
	if got.Description != app.Description {
		t.Errorf("Description: got %q, want %q", got.Description, app.Description)
	}
	if got.Icon != app.Icon {
		t.Errorf("Icon: got %q, want %q", got.Icon, app.Icon)
	}
	if len(got.Tags) != len(app.Tags) {
		t.Errorf("Tags length: got %d, want %d", len(got.Tags), len(app.Tags))
	} else {
		for i, tag := range app.Tags {
			if got.Tags[i] != tag {
				t.Errorf("Tags[%d]: got %q, want %q", i, got.Tags[i], tag)
			}
		}
	}
	if got.Enabled != app.Enabled {
		t.Errorf("Enabled: got %v, want %v", got.Enabled, app.Enabled)
	}
	if got.Health != app.Health {
		t.Errorf("Health: got %q, want %q", got.Health, app.Health)
	}
}

// TestSlugUniqueness verifies that inserting two apps with the same slug returns an error
// containing "UNIQUE".
func TestSlugUniqueness(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first := newTestApp("duplicate")
	if err := s.CreateApp(ctx, first); err != nil {
		t.Fatalf("CreateApp first: %v", err)
	}

	second := newTestApp("duplicate")
	second.ID = "different-id"
	err := s.CreateApp(ctx, second)
	if err == nil {
		t.Fatal("expected error for duplicate slug, got nil")
	}
	if !strings.Contains(err.Error(), "UNIQUE") {
		t.Errorf("expected error containing 'UNIQUE', got: %v", err)
	}
}

// TestUpdateAppPartial verifies that a partial update only changes the patched field
// and leaves other fields unchanged.
func TestUpdateAppPartial(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	app := newTestApp("partial")
	if err := s.CreateApp(ctx, app); err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}

	newName := "Updated Name"
	updated, err := s.UpdateApp(ctx, app.Slug, AppPatch{Name: &newName})
	if err != nil {
		t.Fatalf("UpdateApp error: %v", err)
	}

	if updated.Name != newName {
		t.Errorf("Name after update: got %q, want %q", updated.Name, newName)
	}
	// All other fields must be unchanged.
	if updated.UpstreamURL != app.UpstreamURL {
		t.Errorf("UpstreamURL changed unexpectedly: got %q, want %q", updated.UpstreamURL, app.UpstreamURL)
	}
	if updated.Slug != app.Slug {
		t.Errorf("Slug changed unexpectedly: got %q, want %q", updated.Slug, app.Slug)
	}
	if updated.DisplayName != app.DisplayName {
		t.Errorf("DisplayName changed unexpectedly: got %q, want %q", updated.DisplayName, app.DisplayName)
	}
	if updated.Enabled != app.Enabled {
		t.Errorf("Enabled changed unexpectedly: got %v, want %v", updated.Enabled, app.Enabled)
	}
}

// TestDeleteAppNotFound verifies that deleting a non-existent slug returns ErrNotFound.
func TestDeleteAppNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.DeleteApp(ctx, "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestGetSettingsDefaults verifies that a fresh database returns the expected default settings.
func TestGetSettingsDefaults(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	settings, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings error: %v", err)
	}

	if settings.TailscaleHostname != "oasis" {
		t.Errorf("TailscaleHostname: got %q, want %q", settings.TailscaleHostname, "oasis")
	}
	if settings.MgmtPort != 4515 {
		t.Errorf("MgmtPort: got %d, want %d", settings.MgmtPort, 4515)
	}
	if settings.Theme != "system" {
		t.Errorf("Theme: got %q, want %q", settings.Theme, "system")
	}
}
