package cli

import (
	"fmt"
	"net/url"
	"os"
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

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive first-time setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, advanced)
		},
	}

	cmd.Flags().BoolVar(&advanced, "advanced", false, "Show advanced configuration options")

	return cmd
}

func runInit(cmd *cobra.Command, advanced bool) error {
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

	image := "ghcr.io/prettysmartdev/oasis:latest"
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

	// Step 4: Pull image.
	if err := table.Spinner("Pulling oasis image...", func() error {
		return docker.PullImage(image, os.Stderr)
	}); err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}

	// Step 5: Run container.
	opts := docker.RunOptions{
		Name:       containerName,
		Image:      image,
		Port:       mgmtPort,
		TsAuthKey:  tsAuthKey,
		TsHostname: tsHostname,
		MgmtPort:   mgmtPort,
	}
	if err := docker.RunContainer(opts); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	// Step 6: Poll for ready.
	endpoint := fmt.Sprintf("http://127.0.0.1:%s", mgmtPort)
	c := client.New(endpoint, cliVersion).WithTimeout(5 * time.Second)

	type statusResp struct {
		TailscaleConnected bool   `json:"tailscaleConnected"`
		TailscaleHostname  string `json:"tailscaleHostname"`
		TailnetName        string `json:"tailnetName"`
		Version            string `json:"version"`
	}

	var sr statusResp
	ready := false

	_ = table.Spinner("Waiting for Tailscale connection...", func() error {
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			var s statusResp
			if err := c.Get("/api/v1/status", &s); err == nil && s.TailscaleConnected {
				sr = s
				ready = true
				return nil
			}
			time.Sleep(2 * time.Second)
		}
		return nil
	})

	if !ready {
		fmt.Fprintln(cmd.OutOrStdout(), "Tailscale connection is taking longer than expected. Your oasis container is running but may not be reachable yet. Check status with `oasis status`.")
	}

	// Step 7: Write config.
	cfg := &config.Config{
		MgmtEndpoint:     endpoint,
		ContainerName:    containerName,
		LastKnownVersion: sr.Version,
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// Step 8: Print success.
	if ready {
		hostname := sr.TailscaleHostname
		if hostname == "" {
			hostname = tsHostname
		}
		tailnet := sr.TailnetName
		var tsURL string
		if tailnet != "" {
			tsURL = fmt.Sprintf("https://%s.%s.ts.net", hostname, tailnet)
		} else {
			tsURL = fmt.Sprintf("https://%s.ts.net", hostname)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Your oasis is ready at %s\n", tsURL)
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
