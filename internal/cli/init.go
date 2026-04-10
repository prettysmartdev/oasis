package cli

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/prettysmartdev/oasis/internal/cli/client"
	"github.com/prettysmartdev/oasis/internal/cli/config"
	"github.com/prettysmartdev/oasis/internal/cli/docker"
	"github.com/prettysmartdev/oasis/internal/cli/table"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newInitCmd returns the `oasis init` interactive setup command.
func newInitCmd() *cobra.Command {
	var advanced bool
	var dev bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive first-time setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, advanced, dev)
		},
	}

	cmd.Flags().BoolVar(&advanced, "advanced", false, "Show advanced configuration options")
	cmd.Flags().BoolVar(&dev, "dev", false, "Use locally built dev image (oasis:latest) instead of pulling from registry")

	return cmd
}

func runInit(cmd *cobra.Command, advanced bool, dev bool) error {
	// Step 1: Check docker is available.
	if _, err := docker.ContainerExists("__probe__"); err != nil {
		if strings.Contains(err.Error(), "not installed") || strings.Contains(err.Error(), "not in PATH") {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		// Other errors (e.g. non-existent container) are expected; ignore.
	}

	// Step 2: Check if already running.
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	if _, err := os.Stat(cfgPath); err == nil {
		cfg, _ := config.Load(cfgPath)
		if cfg != nil {
			running, _ := docker.ContainerRunning(cfg.ContainerName)
			if running {
				fmt.Fprintln(cmd.OutOrStdout(), "oasis is already running. Use `oasis status` to check.")
				return nil
			}
			// Config exists but not running.
			fmt.Fprintln(cmd.OutOrStdout(), "A previous oasis setup was found but the container is not running.")
			fmt.Fprintln(cmd.OutOrStdout(), "Run `oasis start` to start it, or continue to re-initialise.")
		}
	}

	// Step 3: Prompt for configuration.
	tsAuthKey, err := promptPassword("Tailscale auth key (https://login.tailscale.com/admin/settings/keys): ")
	if err != nil {
		return fmt.Errorf("reading auth key: %w", err)
	}

	tsHostname := promptStringDefault("Tailscale hostname", "oasis")

	mgmtPort := "04515"
	if advanced {
		mgmtPort = promptStringDefault("Management port", "04515")
	}

	// Detect claude config on host.
	home, _ := os.UserHomeDir()
	claudeJSONPath := filepath.Join(home, ".claude.json")
	claudeDirPath := filepath.Join(home, ".claude")
	hasClaudeJSON := fileExists(claudeJSONPath)
	hasClaudeDir := dirExists(claudeDirPath)

	// Offer Claude authentication.
	claudeOAuthToken := ""
	if promptYesNo("Configure Claude authentication now?", true) {
		claudeOAuthToken = setupClaudeAuth()
	}

	const devImage = "oasis:latest"
	const remoteImage = "ghcr.io/prettysmartdev/oasis:latest"

	image := remoteImage
	if dev {
		image = devImage
	}
	containerName := "oasis"

	// Handle stale container.
	exists, _ := docker.ContainerExists(containerName)
	if exists {
		fmt.Fprintf(cmd.OutOrStdout(), "A container named %q already exists. Removing it before proceeding.\n", containerName)
		_ = docker.StopContainer(containerName)
		if err := docker.RemoveContainer(containerName); err != nil {
			return fmt.Errorf("removing stale container: %w", err)
		}
	}

	// Step 4: Pull image (skipped in dev mode — uses locally built image).
	if !dev {
		if err := table.Spinner("Pulling oasis image...", func() error {
			return docker.PullImage(image, os.Stderr)
		}); err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
	}

	// Step 5: Run container.
	opts := docker.RunOptions{
		Name:           containerName,
		Image:          image,
		Port:           mgmtPort,
		TsAuthKey:      tsAuthKey,
		TsHostname:     tsHostname,
		MgmtPort:       mgmtPort,
		ClaudeJSONPath: claudeJSONPath,
		ClaudeDirPath:  claudeDirPath,
		MountClaude:    hasClaudeJSON && hasClaudeDir,
	}
	if err := docker.RunContainer(opts); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%s", mgmtPort)

	type statusResp struct {
		TailscaleConnected bool   `json:"tailscaleConnected"`
		TailscaleHostname  string `json:"tailscaleHostname"`
		TailscaleDNSName   string `json:"tailscaleDNSName"`
		Version            string `json:"version"`
	}

	// Step 6: Wait for the management API to be reachable (controller may take a moment to start).
	apiReady := false
	_ = table.Spinner("Waiting for controller to start...", func() error {
		probe := client.New(endpoint, cliVersion).WithTimeout(3 * time.Second)
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			var s statusResp
			if err := probe.Get("/api/v1/status", &s); err == nil {
				apiReady = true
				return nil
			}
			time.Sleep(1 * time.Second)
		}
		return nil
	})

	if !apiReady {
		return fmt.Errorf("controller did not start within 30 seconds — check logs with `oasis logs`")
	}

	// Step 7: Drive Tailscale setup via the management API.
	// POST /api/v1/setup passes the auth key to the controller and blocks until the
	// tsnet node has joined the tailnet. Use a long timeout — auth can take ~30 s.
	var sr statusResp
	ready := false

	_ = table.Spinner("Connecting to Tailscale...", func() error {
		setupClient := client.New(endpoint, cliVersion).WithTimeout(90 * time.Second)
		body := map[string]any{
			"tailscaleAuthKey":   tsAuthKey,
			"hostname":           tsHostname,
			"claude_oauth_token": claudeOAuthToken,
		}
		if err := setupClient.Post("/api/v1/setup", body, &sr); err != nil {
			return err
		}
		ready = sr.TailscaleConnected
		return nil
	})

	if !ready {
		fmt.Fprintln(cmd.OutOrStdout(), "Tailscale connection is taking longer than expected. Your oasis container is running but may not be reachable yet. Check status with `oasis status`.")
	}

	// Step 8: Write config.
	cfg := &config.Config{
		MgmtEndpoint:     endpoint,
		ContainerName:    containerName,
		LastKnownVersion: sr.Version,
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// Step 9: Print success.
	if ready {
		hostname := sr.TailscaleHostname
		if hostname == "" {
			hostname = tsHostname
		}
		oasisURL := sr.TailscaleDNSName
		if oasisURL == "" {
			// Fallback: best-effort URL if the controller didn't return a DNS name.
			oasisURL = hostname + ".ts.net"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Your oasis is ready at https://%s\n", oasisURL)
	}

	// Warn if claude auth paths were not mounted.
	if !hasClaudeJSON || !hasClaudeDir {
		fmt.Fprintln(os.Stderr, "\nWarning: ~/.claude.json and/or ~/.claude/ were not found on this host.")
		fmt.Fprintln(os.Stderr, "Claude features inside oasis require authentication. To complete setup, run:")
		fmt.Fprintln(os.Stderr, "\n  docker exec -it oasis claude")
	}

	return nil
}

// promptPassword reads a masked password from the terminal (without echo).
func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // newline after masked input
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	// Fallback for non-TTY (e.g. piped input).
	var s string
	_, err := fmt.Fscanln(os.Stdin, &s)
	return strings.TrimSpace(s), err
}

// promptStringDefault prompts the user for a string with a default value.
func promptStringDefault(label, defaultVal string) string {
	fmt.Fprintf(os.Stderr, "%s [%s]: ", label, defaultVal)
	var s string
	fmt.Fscanln(os.Stdin, &s)
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal
	}
	return s
}

// isValidURL checks whether s is a valid http or https URL.
func isValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// promptYesNo prompts the user with a yes/no question. defaultYes controls
// whether pressing Enter (empty input) is treated as yes.
func promptYesNo(question string, defaultYes bool) bool {
	if defaultYes {
		fmt.Fprintf(os.Stderr, "%s [Y/n]: ", question)
	} else {
		fmt.Fprintf(os.Stderr, "%s [y/N]: ", question)
	}
	var s string
	fmt.Fscanln(os.Stdin, &s)
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return defaultYes
	}
	return s == "y" || s == "yes"
}

// setupClaudeAuth runs `claude setup-token` interactively and returns the token
// the user pastes. Returns an empty string if claude is not found or the user
// skips token entry.
func setupClaudeAuth() string {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: 'claude' binary not found on PATH. Skipping Claude authentication setup.")
		fmt.Fprintln(os.Stderr, "You can configure this later by re-running 'oasis init' or setting the token manually.")
		return ""
	}

	// Run claude setup-token with terminal I/O connected.
	setupCmd := exec.Command(claudePath, "setup-token")
	setupCmd.Stdin = os.Stdin
	setupCmd.Stdout = os.Stdout
	setupCmd.Stderr = os.Stderr
	_ = setupCmd.Run() // ignore exit code — some platforms exit non-zero

	fmt.Fprint(os.Stderr, "Paste the OAuth token you copied: ")
	var token string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err == nil {
			token = strings.TrimSpace(string(b))
		}
	} else {
		fmt.Fscanln(os.Stdin, &token)
		token = strings.TrimSpace(token)
	}

	if token == "" {
		fmt.Fprintln(os.Stderr, "Warning: No token provided. Claude features will be unavailable until a token is configured.")
	}
	return token
}
