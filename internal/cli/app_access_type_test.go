package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// resetCmdFlag resets a cobra flag to its default value to prevent pflag value
// contamination between test runs. cobra/pflag does not reset flag values
// automatically between Execute() calls in the same process.
func resetCmdFlag(t *testing.T, cmdPath []string, flagName string) {
	t.Helper()
	cmd, _, err := rootCmd.Find(cmdPath)
	if err != nil || cmd == nil {
		return
	}
	if f := cmd.Flags().Lookup(flagName); f != nil {
		_ = f.Value.Set(f.DefValue)
	}
}

// TestAppAddAccessTypeFlagRegistered verifies that the "app add" command exposes
// an --access-type flag defaulting to "proxy".
func TestAppAddAccessTypeFlagRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"app", "add"})
	if err != nil || cmd == nil || cmd.Name() != "add" {
		t.Fatalf("could not find 'app add' command: err=%v cmd=%v", err, cmd)
	}
	f := cmd.Flags().Lookup("access-type")
	if f == nil {
		t.Fatal("expected --access-type flag on 'app add', got nil")
	}
	if f.DefValue != "proxy" {
		t.Errorf("--access-type default: got %q, want %q", f.DefValue, "proxy")
	}
}

// TestAppUpdateAccessTypeFlagRegistered verifies that the "app update" command exposes
// an --access-type flag.
func TestAppUpdateAccessTypeFlagRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"app", "update"})
	if err != nil || cmd == nil || cmd.Name() != "update" {
		t.Fatalf("could not find 'app update' command: err=%v cmd=%v", err, cmd)
	}
	f := cmd.Flags().Lookup("access-type")
	if f == nil {
		t.Fatal("expected --access-type flag on 'app update', got nil")
	}
}

// TestAppAddProxySendsCorrectAccessType verifies that `oasis app add --access-type proxy`
// sends accessType:"proxy" in the POST body.
func TestAppAddProxySendsCorrectAccessType(t *testing.T) {
	// Reset the --file flag so a previous test's temp path doesn't contaminate this run.
	resetCmdFlag(t, []string{"app", "add"}, "file")

	var gotBody map[string]any

	cfgPath := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/apps" && r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"1","name":"X","slug":"x-app","upstreamURL":"http://localhost:3000","accessType":"proxy","health":"unknown","enabled":true,"tags":[],"displayName":"","description":"","icon":"","createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-01-01T00:00:00Z"}`)) //nolint:errcheck
			return
		}
		http.NotFound(w, r)
	})

	_, _, err := runCLI(t,
		"--config", cfgPath,
		"app", "add",
		"--name", "X",
		"--slug", "x-app",
		"--url", "http://localhost:3000",
		"--access-type", "proxy",
	)
	if err != nil {
		t.Fatalf("app add error: %v", err)
	}
	if gotBody == nil {
		t.Fatal("no POST request was made to /api/v1/apps")
	}
	if gotBody["accessType"] != "proxy" {
		t.Errorf("accessType in POST body: got %v, want %q", gotBody["accessType"], "proxy")
	}
}

// TestAppUpdateSendsAccessTypeOnPatch verifies that `oasis app update --access-type direct`
// sends accessType:"direct" in the PATCH body.
func TestAppUpdateSendsAccessTypeOnPatch(t *testing.T) {
	var gotBody map[string]any

	cfgPath := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/apps/x-app" && r.Method == http.MethodPatch {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`)) //nolint:errcheck
			return
		}
		http.NotFound(w, r)
	})

	_, _, err := runCLI(t,
		"--config", cfgPath,
		"app", "update", "x-app",
		"--access-type", "direct",
	)
	if err != nil {
		t.Fatalf("app update error: %v", err)
	}
	if gotBody == nil {
		t.Fatal("no PATCH request was made to /api/v1/apps/x-app")
	}
	if gotBody["accessType"] != "direct" {
		t.Errorf("accessType in PATCH body: got %v, want %q", gotBody["accessType"], "direct")
	}
}

// TestAppAddDefaultsToProxy verifies that `oasis app add` without --access-type sends
// accessType:"proxy" (the flag's default value) in the POST body.
func TestAppAddDefaultsToProxy(t *testing.T) {
	// Reset the --file flag so a previous test's temp path doesn't contaminate this run.
	resetCmdFlag(t, []string{"app", "add"}, "file")

	var gotBody map[string]any

	cfgPath := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/apps" && r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"2","name":"Default App","slug":"default-app","upstreamURL":"http://localhost:4000","accessType":"proxy","health":"unknown","enabled":true,"tags":[],"displayName":"","description":"","icon":"","createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-01-01T00:00:00Z"}`)) //nolint:errcheck
			return
		}
		http.NotFound(w, r)
	})

	// No --access-type flag provided; the flag default of "proxy" should be used.
	_, _, err := runCLI(t,
		"--config", cfgPath,
		"app", "add",
		"--name", "Default App",
		"--slug", "default-app",
		"--url", "http://localhost:4000",
	)
	if err != nil {
		t.Fatalf("app add error: %v", err)
	}
	if gotBody == nil {
		t.Fatal("no POST request was made to /api/v1/apps")
	}
	if gotBody["accessType"] != "proxy" {
		t.Errorf("accessType default in POST body: got %v, want %q", gotBody["accessType"], "proxy")
	}
	// Also confirm the sent value is not an empty string.
	if strings.TrimSpace(gotBody["accessType"].(string)) == "" {
		t.Error("accessType must not be empty in the POST body")
	}
}

// TestAppAddInvalidAccessTypeExits2 verifies that `oasis app add --access-type invalid`
// exits with code 2, prints an error to stderr, and makes no API call.
//
// os.Exit cannot be tested in-process without killing the test binary, so this
// test spawns a subprocess of the test binary itself (the standard Go pattern).
func TestAppAddInvalidAccessTypeExits2(t *testing.T) {
	// Subprocess sentinel: when set, this invocation runs the CLI and is
	// expected to call os.Exit(2) before making any API request.
	if os.Getenv("OASIS_TEST_INVALID_AT") == "1" {
		cfgPath := os.Getenv("OASIS_TEST_CFG")
		rootCmd.SetArgs([]string{
			"--config", cfgPath,
			"app", "add",
			"--name", "X",
			"--slug", "x-invalid",
			"--url", "http://localhost:3000",
			"--access-type", "invalid",
		})
		_ = rootCmd.Execute()
		return
	}

	// Parent process: start a test server that records whether it was called,
	// then spawn the subprocess.
	apiCalled := false
	cfgPath := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		http.Error(w, "should not be reached", http.StatusInternalServerError)
	})

	cmd := exec.Command(os.Args[0], "-test.run=TestAppAddInvalidAccessTypeExits2")
	cmd.Env = append(os.Environ(),
		"OASIS_TEST_INVALID_AT=1",
		"OASIS_TEST_CFG="+cfgPath,
	)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if e, ok := err.(*exec.ExitError); ok {
		exitCode = e.ExitCode()
	}

	if exitCode != 2 {
		t.Errorf("exit code: got %d, want 2", exitCode)
	}
	if !strings.Contains(stderrBuf.String(), "must be one of") {
		t.Errorf("expected error message on stderr, got: %q", stderrBuf.String())
	}
	if apiCalled {
		t.Error("API must not be called when --access-type validation fails")
	}
}
