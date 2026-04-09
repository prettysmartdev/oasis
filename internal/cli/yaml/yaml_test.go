package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// --- ParseAppFile tests ------------------------------------------------------

// TestParseAppFileValid verifies a fully-populated app YAML parses without error.
func TestParseAppFileValid(t *testing.T) {
	content := `
name: "My App"
slug: "my-app"
upstreamUrl: "http://localhost:3000"
description: "A test app"
icon: "🚀"
tags:
  - work
  - tools
`
	path := writeTemp(t, content)
	def, err := ParseAppFile(path)
	if err != nil {
		t.Fatalf("ParseAppFile error: %v", err)
	}
	if def.Name != "My App" {
		t.Errorf("Name: got %q, want %q", def.Name, "My App")
	}
	if def.Slug != "my-app" {
		t.Errorf("Slug: got %q, want %q", def.Slug, "my-app")
	}
	if def.UpstreamURL != "http://localhost:3000" {
		t.Errorf("UpstreamURL: got %q, want %q", def.UpstreamURL, "http://localhost:3000")
	}
	if len(def.Tags) != 2 {
		t.Errorf("Tags: got %v, want 2 tags", def.Tags)
	}
}

// TestParseAppFileMissingAllRequired verifies errors on all three missing required fields at once.
func TestParseAppFileMissingAllRequired(t *testing.T) {
	content := `description: "no name, no slug, no url"`
	path := writeTemp(t, content)
	_, err := ParseAppFile(path)
	if err == nil {
		t.Fatal("expected error for missing required fields, got nil")
	}
	msg := err.Error()
	for _, field := range []string{"name", "slug", "upstreamUrl"} {
		if !strings.Contains(msg, field) {
			t.Errorf("error message %q does not mention missing field %q", msg, field)
		}
	}
}

// TestParseAppFileMissingName verifies name is reported as missing.
func TestParseAppFileMissingName(t *testing.T) {
	content := `
slug: "my-app"
upstreamUrl: "http://localhost:3000"
`
	path := writeTemp(t, content)
	_, err := ParseAppFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name': %v", err)
	}
}

// TestParseAppFileMissingSlug verifies slug is reported as missing.
func TestParseAppFileMissingSlug(t *testing.T) {
	content := `
name: "My App"
upstreamUrl: "http://localhost:3000"
`
	path := writeTemp(t, content)
	_, err := ParseAppFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "slug") {
		t.Errorf("error should mention 'slug': %v", err)
	}
}

// TestParseAppFileMissingURL verifies upstreamUrl is reported as missing.
func TestParseAppFileMissingURL(t *testing.T) {
	content := `
name: "My App"
slug: "my-app"
`
	path := writeTemp(t, content)
	_, err := ParseAppFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upstreamUrl") {
		t.Errorf("error should mention 'upstreamUrl': %v", err)
	}
}

// TestParseAppFileNotExist verifies an error when the file doesn't exist.
func TestParseAppFileNotExist(t *testing.T) {
	_, err := ParseAppFile("/tmp/does-not-exist-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// --- ParseAgentFile tests -----------------------------------------------------

// TestParseAgentFileValid verifies a fully-populated agent YAML parses without error.
func TestParseAgentFileValid(t *testing.T) {
	content := `
name: "Daily Digest"
slug: "daily-digest"
description: "Summarise the news"
icon: "📰"
prompt: "Summarise the news today."
trigger: "schedule"
schedule: "0 8 * * *"
outputFmt: "markdown"
`
	path := writeTemp(t, content)
	def, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("ParseAgentFile error: %v", err)
	}
	if def.Name != "Daily Digest" {
		t.Errorf("Name: got %q, want %q", def.Name, "Daily Digest")
	}
	if def.Slug != "daily-digest" {
		t.Errorf("Slug: got %q, want %q", def.Slug, "daily-digest")
	}
	if def.Trigger != "schedule" {
		t.Errorf("Trigger: got %q, want %q", def.Trigger, "schedule")
	}
	if def.Schedule != "0 8 * * *" {
		t.Errorf("Schedule: got %q, want %q", def.Schedule, "0 8 * * *")
	}
	if def.OutputFmt != "markdown" {
		t.Errorf("OutputFmt: got %q, want %q", def.OutputFmt, "markdown")
	}
}

// TestParseAgentFileMissingAllRequired verifies all missing required fields are listed at once.
func TestParseAgentFileMissingAllRequired(t *testing.T) {
	content := `description: "no required fields"`
	path := writeTemp(t, content)
	_, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, field := range []string{"name", "slug", "prompt", "trigger"} {
		if !strings.Contains(msg, field) {
			t.Errorf("error %q does not mention missing field %q", msg, field)
		}
	}
}

// TestParseAgentFileMissingName verifies name is reported.
func TestParseAgentFileMissingName(t *testing.T) {
	content := `
slug: "my-agent"
prompt: "do something"
trigger: "tap"
`
	path := writeTemp(t, content)
	_, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name': %v", err)
	}
}

// TestParseAgentFileMissingSlug verifies slug is reported.
func TestParseAgentFileMissingSlug(t *testing.T) {
	content := `
name: "My Agent"
prompt: "do something"
trigger: "tap"
`
	path := writeTemp(t, content)
	_, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "slug") {
		t.Errorf("error should mention 'slug': %v", err)
	}
}

// TestParseAgentFileMissingPrompt verifies prompt is reported.
func TestParseAgentFileMissingPrompt(t *testing.T) {
	content := `
name: "My Agent"
slug: "my-agent"
trigger: "tap"
`
	path := writeTemp(t, content)
	_, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "prompt") {
		t.Errorf("error should mention 'prompt': %v", err)
	}
}

// TestParseAgentFileMissingTrigger verifies trigger is reported.
func TestParseAgentFileMissingTrigger(t *testing.T) {
	content := `
name: "My Agent"
slug: "my-agent"
prompt: "do something"
`
	path := writeTemp(t, content)
	_, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "trigger") {
		t.Errorf("error should mention 'trigger': %v", err)
	}
}

// TestParseAgentFileScheduleTriggerMissingSchedule verifies trigger=schedule with empty schedule errors.
func TestParseAgentFileScheduleTriggerMissingSchedule(t *testing.T) {
	content := `
name: "Sched Agent"
slug: "sched-agent"
prompt: "do something"
trigger: "schedule"
`
	path := writeTemp(t, content)
	_, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("expected error for schedule trigger with missing schedule, got nil")
	}
	if !strings.Contains(err.Error(), "schedule") {
		t.Errorf("error should mention 'schedule': %v", err)
	}
}

// TestParseAgentFileOutputFmtDefault verifies outputFmt defaults to "markdown" when omitted.
func TestParseAgentFileOutputFmtDefault(t *testing.T) {
	content := `
name: "My Agent"
slug: "my-agent"
prompt: "do something"
trigger: "tap"
`
	path := writeTemp(t, content)
	def, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("ParseAgentFile error: %v", err)
	}
	if def.OutputFmt != "markdown" {
		t.Errorf("OutputFmt default: got %q, want %q", def.OutputFmt, "markdown")
	}
}

// TestParseAgentFileOutputFmtExplicit verifies an explicit outputFmt is preserved.
func TestParseAgentFileOutputFmtExplicit(t *testing.T) {
	content := `
name: "My Agent"
slug: "my-agent"
prompt: "do something"
trigger: "tap"
outputFmt: "html"
`
	path := writeTemp(t, content)
	def, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("ParseAgentFile error: %v", err)
	}
	if def.OutputFmt != "html" {
		t.Errorf("OutputFmt: got %q, want %q", def.OutputFmt, "html")
	}
}

// --- accessType field tests for ParseAppFile ----------------------------------

// TestParseAppFileAccessTypeProxy verifies that an explicit accessType:"proxy" is preserved.
func TestParseAppFileAccessTypeProxy(t *testing.T) {
	content := `
name: "Proxy App"
slug: "proxy-app"
upstreamUrl: "http://localhost:3000"
accessType: "proxy"
`
	path := writeTemp(t, content)
	def, err := ParseAppFile(path)
	if err != nil {
		t.Fatalf("ParseAppFile error: %v", err)
	}
	if def.AccessType != "proxy" {
		t.Errorf("AccessType: got %q, want %q", def.AccessType, "proxy")
	}
}

// TestParseAppFileAccessTypeInvalid verifies that an unrecognised accessType value
// returns an error that mentions "accessType".
func TestParseAppFileAccessTypeInvalid(t *testing.T) {
	content := `
name: "Bad App"
slug: "bad-app"
upstreamUrl: "http://localhost:3000"
accessType: "invalid"
`
	path := writeTemp(t, content)
	_, err := ParseAppFile(path)
	if err == nil {
		t.Fatal("expected error for invalid accessType, got nil")
	}
	if !strings.Contains(err.Error(), "accessType") {
		t.Errorf("error should mention 'accessType': %v", err)
	}
}

// TestParseAppFileAccessTypeOmittedDefaultsToProxy verifies that omitting the accessType
// field causes ParseAppFile to default it to "proxy".
func TestParseAppFileAccessTypeOmittedDefaultsToProxy(t *testing.T) {
	content := `
name: "No Access Type App"
slug: "no-access-type-app"
upstreamUrl: "http://localhost:3000"
`
	path := writeTemp(t, content)
	def, err := ParseAppFile(path)
	if err != nil {
		t.Fatalf("ParseAppFile error: %v", err)
	}
	if def.AccessType != "proxy" {
		t.Errorf("AccessType default: got %q, want %q", def.AccessType, "proxy")
	}
}
