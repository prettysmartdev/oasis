// Package agent manages the agent run lifecycle and defines the AgentHarness interface
// that all AI backend implementations must satisfy.
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// AgentHarness runs an agent's prompt in a pre-created work directory.
// Implementations are expected to write their output to a file in workDir
// as instructed by the system prompt; the caller reads that file after Execute returns.
type AgentHarness interface {
	// Execute runs the agent and returns when the process has finished or ctx is cancelled.
	// workDir is the absolute path to a pre-created directory dedicated to this run.
	// The AgentHarness must set the subprocess CWD to workDir.
	Execute(ctx context.Context, a db.Agent, workDir string) error
}

// OutputFilename returns the result file name for the given output format.
// "markdown" → "result.md", "html" → "result.html", anything else → "result.txt".
func OutputFilename(outputFmt string) string {
	switch outputFmt {
	case "html":
		return "result.html"
	case "plaintext":
		return "result.txt"
	default:
		return "result.md"
	}
}

// StubHarness is an AgentHarness implementation that writes pre-formatted stub
// output to the output file without calling any real AI backend. It is used
// when no harness has been configured (e.g. in tests or before claude is set up).
type StubHarness struct{}

// Execute writes stub output to the appropriate file in workDir.
func (StubHarness) Execute(_ context.Context, a db.Agent, workDir string) error {
	content := stubContent(a)
	if content == "" {
		return fmt.Errorf("unknown output format: %s", a.OutputFmt)
	}
	path := filepath.Join(workDir, OutputFilename(a.OutputFmt))
	return os.WriteFile(path, []byte(content), 0o644)
}

func stubContent(a db.Agent) string {
	switch a.OutputFmt {
	case "markdown":
		return fmt.Sprintf("# Agent: %s\n\n> **Prompt:** %s\n\n_Stub output — AI backend not yet connected._\n\n- Item one\n- Item two\n", a.Name, a.Prompt)
	case "html":
		return fmt.Sprintf(`<h1>Agent: %s</h1><blockquote><strong>Prompt:</strong> %s</blockquote><p><em>Stub output — AI backend not yet connected.</em></p><ul><li>Item one</li><li>Item two</li></ul>`, a.Name, a.Prompt)
	case "plaintext":
		return fmt.Sprintf("Agent: %s\nPrompt: %s\n\nStub output — AI backend not yet connected.\n\n* Item one\n* Item two\n", a.Name, a.Prompt)
	default:
		return ""
	}
}
