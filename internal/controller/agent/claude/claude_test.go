package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prettysmartdev/oasis/internal/controller/agent"
	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// writeFakeClaude writes a shell script fake claude binary to a temp dir.
// It records args (one per line) in args.txt, env in env.txt, pwd in workdir.txt,
// and writes output files (result.md, result.html, result.txt) in its CWD.
// Pass fail=true to make the binary exit 1.
func writeFakeClaude(t *testing.T, fail bool) string {
	t.Helper()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "claude")

	exitCode := "0"
	if fail {
		exitCode = "1"
	}

	script := `#!/bin/sh
printf '%s\n' "$@" > args.txt
env > env.txt
pwd > workdir.txt
touch result.md result.html result.txt
exit ` + exitCode + "\n"

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("writeFakeClaude: %v", err)
	}
	return binPath
}

// containsLine reports whether content contains line as an exact line.
func containsLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if l == line {
			return true
		}
	}
	return false
}

func newTestAgent(model, outputFmt string) db.Agent {
	return db.Agent{
		ID:        "test-id",
		Name:      "Test Agent",
		Slug:      "test-agent",
		Prompt:    "Summarise the news today.",
		Trigger:   "tap",
		OutputFmt: outputFmt,
		Model:     model,
		Enabled:   true,
	}
}

func readWorkFile(t *testing.T, workDir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return strings.TrimRight(string(data), "\n\r ")
}

// TestExecuteAlwaysHasPrintFlag verifies --print is always passed to claude.
func TestExecuteAlwaysHasPrintFlag(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := readWorkFile(t, workDir, "args.txt")
	if !containsLine(content, "--print") {
		t.Errorf("expected --print in args.txt:\n%s", content)
	}
}

// TestExecuteAlwaysHasPermissionModeFlag verifies --permission-mode acceptEdits is always passed.
func TestExecuteAlwaysHasPermissionModeFlag(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := readWorkFile(t, workDir, "args.txt")
	if !containsLine(content, "--permission-mode") {
		t.Errorf("expected --permission-mode in args.txt:\n%s", content)
	}
	if !containsLine(content, "acceptEdits") {
		t.Errorf("expected acceptEdits in args.txt:\n%s", content)
	}
}

// TestExecuteIncludesModelWhenSet verifies --model and value are passed when Model is non-empty.
func TestExecuteIncludesModelWhenSet(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("claude-opus-4-6", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := readWorkFile(t, workDir, "args.txt")
	if !containsLine(content, "--model") {
		t.Errorf("expected --model in args.txt:\n%s", content)
	}
	if !containsLine(content, "claude-opus-4-6") {
		t.Errorf("expected model value in args.txt:\n%s", content)
	}
}

// TestExecuteOmitsModelWhenEmpty verifies --model is NOT passed when Model is empty.
func TestExecuteOmitsModelWhenEmpty(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := readWorkFile(t, workDir, "args.txt")
	if containsLine(content, "--model") {
		t.Errorf("expected --model to be absent in args.txt:\n%s", content)
	}
}

// TestExecuteSystemPromptContainsOutputFilePath verifies the system prompt arg contains
// the expected output file path.
func TestExecuteSystemPromptContainsOutputFilePath(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := readWorkFile(t, workDir, "args.txt")
	expectedPath := filepath.Join(workDir, "result.md")
	if !strings.Contains(content, expectedPath) {
		t.Errorf("expected output file path %q in args.txt:\n%s", expectedPath, content)
	}
}

// TestExecuteSystemPromptContainsAgentPrompt verifies the system prompt arg contains the agent prompt.
func TestExecuteSystemPromptContainsAgentPrompt(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := readWorkFile(t, workDir, "args.txt")
	if !strings.Contains(content, a.Prompt) {
		t.Errorf("expected agent prompt %q in args.txt:\n%s", a.Prompt, content)
	}
}

// TestExecuteSetsWorkDir verifies that the subprocess CWD is set to workDir.
func TestExecuteSetsWorkDir(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	got := readWorkFile(t, workDir, "workdir.txt")
	// Compare base names to handle OS-level symlinks (e.g. /private/var vs /var on macOS).
	if filepath.Base(got) != filepath.Base(workDir) {
		t.Errorf("workdir: got %q, want base %q", got, workDir)
	}
}

// TestExecuteReturnsErrorOnNonZeroExit verifies an error is returned when the binary exits 1.
func TestExecuteReturnsErrorOnNonZeroExit(t *testing.T) {
	fakeBin := writeFakeClaude(t, true)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	err := h.Execute(context.Background(), a, workDir)
	if err == nil {
		t.Error("expected error for non-zero exit, got nil")
	}
}

// TestSystemPromptOutputFilenameMarkdown verifies the output file ends in result.md.
func TestSystemPromptOutputFilenameMarkdown(t *testing.T) {
	name := agent.OutputFilename("markdown")
	if name != "result.md" {
		t.Errorf("OutputFilename(markdown): got %q, want %q", name, "result.md")
	}
}

// TestSystemPromptOutputFilenameHTML verifies the output file ends in result.html.
func TestSystemPromptOutputFilenameHTML(t *testing.T) {
	name := agent.OutputFilename("html")
	if name != "result.html" {
		t.Errorf("OutputFilename(html): got %q, want %q", name, "result.html")
	}
}

// TestSystemPromptOutputFilenameText verifies the output file ends in result.txt.
func TestSystemPromptOutputFilenameText(t *testing.T) {
	name := agent.OutputFilename("plaintext")
	if name != "result.txt" {
		t.Errorf("OutputFilename(plaintext): got %q, want %q", name, "result.txt")
	}
}

// TestNewWithCustomPath verifies New(fakeBin, nil) uses the provided binary path.
func TestNewWithCustomPath(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "args.txt")); err != nil {
		t.Errorf("expected args.txt to be created: %v", err)
	}
}

// TestNewEmptyPathUsesEnvVar verifies New("", nil) falls back to OASIS_CLAUDE_BIN.
func TestNewEmptyPathUsesEnvVar(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	t.Setenv("OASIS_CLAUDE_BIN", fakeBin)
	workDir := t.TempDir()
	h := New("", nil)
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "args.txt")); err != nil {
		t.Errorf("expected args.txt to be created: %v", err)
	}
}

// TestExecuteExtraEnvAppended verifies extra env vars are injected into the subprocess.
func TestExecuteExtraEnvAppended(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	h := New(fakeBin, []string{"CLAUDE_CODE_OAUTH_TOKEN=tok"})
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(workDir, "env.txt"))
	if err != nil {
		t.Fatalf("read env.txt: %v", err)
	}
	if !strings.Contains(string(data), "CLAUDE_CODE_OAUTH_TOKEN=tok") {
		t.Errorf("expected CLAUDE_CODE_OAUTH_TOKEN=tok in env.txt:\n%s", string(data))
	}
}

// TestExecuteOAuthTokenInExtraEnvPresentInSubprocess verifies that a different token
// value also ends up in the subprocess environment.
func TestExecuteOAuthTokenInExtraEnvPresentInSubprocess(t *testing.T) {
	fakeBin := writeFakeClaude(t, false)
	workDir := t.TempDir()
	token := "supersecrettoken123"
	h := New(fakeBin, []string{"CLAUDE_CODE_OAUTH_TOKEN=" + token})
	a := newTestAgent("", "markdown")
	if err := h.Execute(context.Background(), a, workDir); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(workDir, "env.txt"))
	if err != nil {
		t.Fatalf("read env.txt: %v", err)
	}
	if !strings.Contains(string(data), "CLAUDE_CODE_OAUTH_TOKEN="+token) {
		t.Errorf("expected CLAUDE_CODE_OAUTH_TOKEN=%s in env.txt:\n%s", token, string(data))
	}
}
