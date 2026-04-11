// Package claude provides a ClaudeHarness that executes agents via the claude CLI.
package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/prettysmartdev/oasis/internal/controller/agent"
	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// systemPromptTmpl is rendered per-run and passed to claude via --system-prompt.
var systemPromptTmpl = template.Must(template.New("sysprompt").Parse(`You are an assistant that completes a task and then creates a file with the results of that task.

File output format: {{.OutputFmt}}

Write the complete result of the task to: {{.OutputFile}}

Do not create anything else.
You should talk through your thought process and discuss the steps you take, but only output the final task result into the file.
The file you write must be valid {{.OutputFmt}}.
The file must exist when you finish.`))

type systemPromptData struct {
	Prompt     string
	OutputFmt  string
	OutputFile string
}

// AgentHarness invokes the claude CLI to execute an agent.
// The zero value is not usable; use New to construct.
type AgentHarness struct {
	binaryPath string
}

// New returns a ClaudeAgentHarness that invokes the claude binary at the given path.
// Pass an empty binaryPath to resolve "claude" via PATH at call time.
func New(binaryPath string) *AgentHarness {
	if binaryPath == "" {
		// Honour the override env var; fall back to resolving "claude" at exec time.
		if v := os.Getenv("OASIS_CLAUDE_BIN"); v != "" {
			binaryPath = v
		}
	}
	return &AgentHarness{
		binaryPath: binaryPath,
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
	// --dangerously-skip-permissions is used so that agent runs complete without
	// prompting the user to approve each tool call. Runs execute inside the
	// container in an isolated work directory.
	args := []string{
		"--print",
		"--dangerously-skip-permissions",
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
	cmd.Env = os.Environ()

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	cmdLine := strings.Join(append([]string{binary}, args...), " ")
	slog.Info("agent harness: executing claude", "agent", a.Slug, "workDir", workDir, "cmd", cmdLine)

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	slog.Info("agent harness: claude finished", "agent", a.Slug, "exitCode", exitCode)

	// Always write full stdout+stderr for post-run debugging.
	agentOut := filepath.Join(workDir, "agentoutput.txt")
	_ = os.WriteFile(agentOut, combined.Bytes(), 0o644)

	if runErr != nil {
		return fmt.Errorf("claude exited with error: %w", runErr)
	}

	return nil
}
