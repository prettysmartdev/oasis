package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

func newTestAgent(name, outputFmt string) db.Agent {
	return db.Agent{
		ID:        "test-id",
		Name:      name,
		Slug:      "test-slug",
		Prompt:    "Do something useful.",
		Trigger:   "tap",
		OutputFmt: outputFmt,
		Enabled:   true,
	}
}

// TestRunMarkdown verifies markdown output starts with # and is non-empty.
func TestRunMarkdown(t *testing.T) {
	a := newTestAgent("MD Agent", "markdown")
	output, err := Run(context.Background(), a)
	if err != nil {
		t.Fatalf("Run markdown error: %v", err)
	}
	if output == "" {
		t.Error("markdown output must not be empty")
	}
	if !strings.HasPrefix(output, "#") {
		t.Errorf("markdown output must start with '#', got: %q", output[:min(len(output), 20)])
	}
}

// TestRunHTML verifies HTML output contains <h1> and </h1> and is non-empty.
func TestRunHTML(t *testing.T) {
	a := newTestAgent("HTML Agent", "html")
	output, err := Run(context.Background(), a)
	if err != nil {
		t.Fatalf("Run html error: %v", err)
	}
	if output == "" {
		t.Error("html output must not be empty")
	}
	if !strings.Contains(output, "<h1>") {
		t.Errorf("html output must contain '<h1>', got: %s", output)
	}
	if !strings.Contains(output, "</h1>") {
		t.Errorf("html output must contain '</h1>', got: %s", output)
	}
}

// TestRunPlaintext verifies plaintext output contains no HTML tags and is non-empty.
func TestRunPlaintext(t *testing.T) {
	a := newTestAgent("Plain Agent", "plaintext")
	output, err := Run(context.Background(), a)
	if err != nil {
		t.Fatalf("Run plaintext error: %v", err)
	}
	if output == "" {
		t.Error("plaintext output must not be empty")
	}
	if strings.Contains(output, "<") || strings.Contains(output, ">") {
		t.Errorf("plaintext output must not contain HTML tags, got: %s", output)
	}
}

// TestRunUnknownFormat verifies that an unknown outputFmt returns an error.
func TestRunUnknownFormat(t *testing.T) {
	a := newTestAgent("Bad Agent", "xml")
	_, err := Run(context.Background(), a)
	if err == nil {
		t.Fatal("expected error for unknown output format, got nil")
	}
}

// TestRunAllFormatsNonEmpty verifies non-empty output for each valid format.
func TestRunAllFormatsNonEmpty(t *testing.T) {
	formats := []string{"markdown", "html", "plaintext"}
	for _, fmt := range formats {
		t.Run(fmt, func(t *testing.T) {
			a := newTestAgent("Agent "+fmt, fmt)
			output, err := Run(context.Background(), a)
			if err != nil {
				t.Fatalf("Run(%q) error: %v", fmt, err)
			}
			if output == "" {
				t.Errorf("Run(%q) returned empty output", fmt)
			}
		})
	}
}

// TestRunIncludesNameAndPrompt verifies the output includes agent name and prompt.
func TestRunIncludesNameAndPrompt(t *testing.T) {
	a := db.Agent{
		ID:        "id",
		Name:      "MySpecialAgent",
		Slug:      "my-agent",
		Prompt:    "UniquePromptText",
		OutputFmt: "markdown",
	}
	output, err := Run(context.Background(), a)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(output, "MySpecialAgent") {
		t.Errorf("output does not contain agent name %q: %s", "MySpecialAgent", output)
	}
	if !strings.Contains(output, "UniquePromptText") {
		t.Errorf("output does not contain prompt %q: %s", "UniquePromptText", output)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
