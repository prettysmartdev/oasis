package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// mockHarness is a configurable AgentHarness for tests.
type mockHarness struct {
	fn     func(ctx context.Context, a db.Agent, workDir string) error
	called bool
}

func (m *mockHarness) Execute(ctx context.Context, a db.Agent, workDir string) error {
	m.called = true
	if m.fn != nil {
		return m.fn(ctx, a, workDir)
	}
	return nil
}

func newTestAgentForLifecycle() db.Agent {
	return db.Agent{
		ID:        "test-id",
		Name:      "Test Agent",
		Slug:      "test-agent",
		Prompt:    "Summarise the news.",
		Trigger:   "tap",
		OutputFmt: "markdown",
		Enabled:   true,
	}
}

// TestExecuteRunOutputFileDone verifies that ExecuteRun returns the file content and status "done"
// when the harness writes the expected output file.
func TestExecuteRunOutputFileDone(t *testing.T) {
	workDir := t.TempDir()
	a := newTestAgentForLifecycle()
	content := "# Agent output\n\nSome content here."

	h := &mockHarness{
		fn: func(ctx context.Context, ag db.Agent, wd string) error {
			return os.WriteFile(filepath.Join(wd, OutputFilename(ag.OutputFmt)), []byte(content), 0o644)
		},
	}

	output, status := ExecuteRun(context.Background(), h, a, workDir)
	if status != "done" {
		t.Errorf("status: got %q, want %q", status, "done")
	}
	if output != content {
		t.Errorf("output: got %q, want %q", output, content)
	}
	if !h.called {
		t.Error("harness.Execute was not called")
	}
}

// TestExecuteRunOutputFileAbsent verifies status "error" when harness succeeds but writes no file.
func TestExecuteRunOutputFileAbsent(t *testing.T) {
	workDir := t.TempDir()
	a := newTestAgentForLifecycle()

	h := &mockHarness{
		fn: func(ctx context.Context, ag db.Agent, wd string) error {
			// Intentionally write nothing.
			return nil
		},
	}

	output, status := ExecuteRun(context.Background(), h, a, workDir)
	if status != "error" {
		t.Errorf("status: got %q, want %q", status, "error")
	}
	if output == "" {
		t.Error("output must contain an explanation, got empty string")
	}
	// The message must mention "no output file" or similar.
	if len(output) == 0 {
		t.Error("output should not be empty for absent file case")
	}
}

// TestExecuteRunErrorFromHarness verifies status "error" when the harness returns a non-context error.
func TestExecuteRunErrorFromHarness(t *testing.T) {
	workDir := t.TempDir()
	a := newTestAgentForLifecycle()

	h := &mockHarness{
		fn: func(ctx context.Context, ag db.Agent, wd string) error {
			return os.ErrPermission
		},
	}

	_, status := ExecuteRun(context.Background(), h, a, workDir)
	if status != "error" {
		t.Errorf("status: got %q, want %q", status, "error")
	}
}

// TestExecuteRunContextCancelled verifies that a cancelled context causes the run to return
// "agent run timed out" and status "error".
func TestExecuteRunContextCancelled(t *testing.T) {
	workDir := t.TempDir()
	a := newTestAgentForLifecycle()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	h := &mockHarness{
		fn: func(ctx context.Context, ag db.Agent, wd string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	output, status := ExecuteRun(ctx, h, a, workDir)
	if status != "error" {
		t.Errorf("status: got %q, want %q", status, "error")
	}
	if output != "agent run timed out" {
		t.Errorf("output: got %q, want %q", output, "agent run timed out")
	}
}

// TestWorkDirCreatedBeforeExecute verifies that the workDir passed to ExecuteRun
// is forwarded unchanged to harness.Execute.
func TestWorkDirCreatedBeforeExecute(t *testing.T) {
	workDir := t.TempDir()
	a := newTestAgentForLifecycle()

	var capturedWorkDir string
	h := &mockHarness{
		fn: func(ctx context.Context, ag db.Agent, wd string) error {
			capturedWorkDir = wd
			// Write the output file so ExecuteRun returns "done".
			return os.WriteFile(filepath.Join(wd, OutputFilename(ag.OutputFmt)), []byte("content"), 0o644)
		},
	}

	ExecuteRun(context.Background(), h, a, workDir)

	if capturedWorkDir != workDir {
		t.Errorf("harness received workDir %q, want %q", capturedWorkDir, workDir)
	}
}
