// Package agent provides stub agent execution for the oasis controller.
// Real AI backends are out of scope for this work item.
package agent

import (
	"context"
	"fmt"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// Run executes the agent using stub output (no real AI backend).
// The output format is determined by a.OutputFmt.
func Run(ctx context.Context, a db.Agent) (string, error) {
	switch a.OutputFmt {
	case "markdown":
		return fmt.Sprintf("# Agent: %s\n\n> **Prompt:** %s\n\n_Stub output — AI backend not yet connected._\n\n- Item one\n- Item two\n", a.Name, a.Prompt), nil
	case "html":
		return fmt.Sprintf(`<h1>Agent: %s</h1><blockquote><strong>Prompt:</strong> %s</blockquote><p><em>Stub output — AI backend not yet connected.</em></p><ul><li>Item one</li><li>Item two</li></ul>`, a.Name, a.Prompt), nil
	case "plaintext":
		return fmt.Sprintf("Agent: %s\nPrompt: %s\n\nStub output — AI backend not yet connected.\n\n* Item one\n* Item two\n", a.Name, a.Prompt), nil
	}
	return "", fmt.Errorf("unknown output format: %s", a.OutputFmt)
}
