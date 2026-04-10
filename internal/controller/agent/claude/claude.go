// Package claude provides a ClaudeHarness that executes agents via the claude CLI.
package claude

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/prettysmartdev/oasis/internal/controller/agent"
	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// systemPromptTmpl is rendered per-run and passed to claude via --system-prompt.
var systemPromptTmpl = template.Must(template.New("sysprompt").Parse(`You are an AI assistant integrated into the Oasis homescreen. Your task is:

{{.Prompt}}

Output format: {{.OutputFmt}}

Write your complete response to the file: {{.OutputFile}}
Do not output anything else. The file you write must be valid {{.OutputFmt}}. The file must exist when you finish.`))

type systemPromptData struct {
	Prompt     string
	OutputFmt  string
	OutputFile string
}

// AgentHarness invokes the claude CLI to execute an agent.
// The zero value is not usable; use New to construct.
type AgentHarness struct {
	binaryPath string
	extraEnv   []string
}

// New returns a ClaudeAgentHarness that invokes the claude binary at the given path.
// Pass an empty binaryPath to resolve "claude" via PATH at call time.
// extraEnv entries (e.g. "CLAUDE_CODE_OAUTH_TOKEN=...") are injected into every subprocess.
func New(binaryPath string, extraEnv []string) *AgentHarness {
	if binaryPath == "" {
		// Honour the override env var; fall back to resolving "claude" at exec time.
		if v := os.Getenv("OASIS_CLAUDE_BIN"); v != "" {
			binaryPath = v
		}
	}
	return &AgentHarness{
		binaryPath: binaryPath,
		extraEnv:   extraEnv,
	}
}

// Execute runs the agent via the claude CLI and returns when the process finishes
// or ctx is cancelled. On success the caller may read the output file. On error
// the combined stdout+stderr is written to error.txt in workDir.
func (h *AgentHarness) Execute(ctx context.Context, a db.Agent, workDir string) error {
	outputFile := filepath.Join(workDir, agent.OutputFilename(a.OutputFmt))

	// Render system prompt.
	var sysBuf strings.Builder
	if err := systemPromptTmpl.Execute(&sysBuf, systemPromptData{
		Prompt:     a.Prompt,
		OutputFmt:  a.OutputFmt,
		OutputFile: outputFile,
	}); err != nil {
		return fmt.Errorf("render system prompt: %w", err)
	}

	// Build args.
	args := []string{
		"--print",
		"--permission-mode", "acceptEdits",
		"--system-prompt", sysBuf.String(),
	}
	if a.Model != "" {
		args = append(args, "--model", a.Model)
	}
	// User prompt is the last positional argument.
	args = append(args, a.Prompt)

	binary := h.binaryPath
	if binary == "" {
		var err error
		binary, err = exec.LookPath("claude")
		if err != nil {
			return fmt.Errorf("claude binary not found on PATH: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), h.extraEnv...)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if err := cmd.Run(); err != nil {
		// Write combined output to error.txt so the caller can store it.
		errTxt := filepath.Join(workDir, "error.txt")
		_ = os.WriteFile(errTxt, combined.Bytes(), 0o644)
		return fmt.Errorf("claude exited with error: %w", err)
	}

	return nil
}
