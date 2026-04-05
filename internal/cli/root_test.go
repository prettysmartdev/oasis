package cli

import (
	"bytes"
	"strings"
	"testing"
)

// resetRootCmd restores rootCmd to a clean state after each test.
// This prevents flag/arg/output state from leaking between tests.
func resetRootCmd(t *testing.T, origVersion string) {
	t.Helper()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		rootCmd.Version = origVersion
	})
}

// TestRootCmdVersion confirms --version prints a non-empty version string.
func TestRootCmdVersion(t *testing.T) {
	orig := rootCmd.Version
	resetRootCmd(t, orig)

	rootCmd.Version = "0.0.1-test"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--version returned unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("--version produced no output")
	}
	if !strings.Contains(output, "0.0.1-test") {
		t.Errorf("--version output %q does not contain version string", output)
	}
}

// TestSubcommandNotImplemented confirms a representative subcommand prints
// "not yet implemented" and exits 0 (returns nil).
func TestSubcommandNotImplemented(t *testing.T) {
	orig := rootCmd.Version
	resetRootCmd(t, orig)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"app", "list"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("'app list' returned unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "not yet implemented") {
		t.Errorf("expected 'not yet implemented', got %q", output)
	}
}

// TestGlobalFlagsRegistered confirms the global flags are present on the root command.
func TestGlobalFlagsRegistered(t *testing.T) {
	flags := []string{"config", "json", "quiet"}
	for _, name := range flags {
		if f := rootCmd.PersistentFlags().Lookup(name); f == nil {
			t.Errorf("expected global flag --%s to be registered", name)
		}
	}
}

// TestAllTopLevelCommandsRegistered confirms all expected top-level commands exist.
func TestAllTopLevelCommandsRegistered(t *testing.T) {
	expected := []string{
		"app", "settings",
		"init", "start", "stop", "restart",
		"status", "update", "logs", "db",
	}
	for _, name := range expected {
		if cmd, _, err := rootCmd.Find([]string{name}); err != nil || cmd.Name() != name {
			t.Errorf("expected command %q to be registered", name)
		}
	}
}

// TestAppSubcommandsRegistered confirms the app subcommand group has all expected children.
func TestAppSubcommandsRegistered(t *testing.T) {
	expected := []string{"add", "list", "show", "remove", "enable", "disable", "update"}
	for _, name := range expected {
		if cmd, _, err := rootCmd.Find([]string{"app", name}); err != nil || cmd.Name() != name {
			t.Errorf("expected 'app %s' subcommand to be registered", name)
		}
	}
}
