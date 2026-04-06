package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/prettysmartdev/oasis/internal/cli/config"
	"github.com/prettysmartdev/oasis/internal/cli/docker"
	"github.com/prettysmartdev/oasis/internal/cli/table"
	"github.com/spf13/cobra"
)

// newStartCmd returns the `oasis start` command.
func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the oasis container",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			exists, err := docker.ContainerExists(cfg.ContainerName)
			if err != nil {
				return err
			}
			if !exists {
				fmt.Fprintln(os.Stderr, "oasis is not initialised — run `oasis init` first")
				os.Exit(1)
			}

			if err := docker.StartContainer(cfg.ContainerName); err != nil {
				return fmt.Errorf("starting container: %w", err)
			}
			if !quiet {
				fmt.Fprintln(cmd.OutOrStdout(), "oasis started.")
			}
			return nil
		},
	}
}

// newStopCmd returns the `oasis stop` command.
func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the oasis container",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := docker.StopContainer(cfg.ContainerName); err != nil {
				return fmt.Errorf("stopping container: %w", err)
			}
			if !quiet {
				fmt.Fprintln(cmd.OutOrStdout(), "oasis stopped.")
			}
			return nil
		},
	}
}

// newRestartCmd returns the `oasis restart` command.
func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the oasis container",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := docker.RestartContainer(cfg.ContainerName); err != nil {
				return fmt.Errorf("restarting container: %w", err)
			}
			if !quiet {
				fmt.Fprintln(cmd.OutOrStdout(), "oasis restarted.")
			}
			return nil
		},
	}
}

// statusAPIResponse mirrors the /api/v1/status response shape.
type statusAPIResponse struct {
	TailscaleConnected bool   `json:"tailscaleConnected"`
	TailscaleHostname  string `json:"tailscaleHostname"`
	NginxStatus        string `json:"nginxStatus"`
	AppsRegistered     int    `json:"appsRegistered"`
	Version            string `json:"version"`
}

// newStatusCmd returns the `oasis status` command.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show controller status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			c := newClient()
			var s statusAPIResponse

			apiErr := c.Get("/api/v1/status", &s)
			if apiErr != nil {
				// Distinguish container stopped vs controller not responding.
				running, _ := docker.ContainerRunning(cfg.ContainerName)
				if !running {
					fmt.Fprintln(cmd.OutOrStdout(), "Container is stopped. Start it with `oasis start`.")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Controller is not responding — check logs with `oasis logs`.")
				}
				os.Exit(1)
			}

			// Version skew warning.
			if s.Version != "" && cliVersion != "" && s.Version != cliVersion {
				fmt.Fprintf(os.Stderr, "Warning: CLI version %s differs from controller version %s. Run `oasis update` to sync.\n", cliVersion, s.Version)
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(s)
			}

			tsStatus := "disconnected"
			if s.TailscaleConnected {
				tsStatus = "connected"
			}

			table.PrintKV([]table.KVPair{
				{Key: "Tailscale", Value: tsStatus},
				{Key: "NGINX", Value: s.NginxStatus},
				{Key: "Apps Registered", Value: strconv.Itoa(s.AppsRegistered)},
				{Key: "Version", Value: s.Version},
			}, cmd.OutOrStdout())

			return nil
		},
	}
}

// newUpdateCmd returns the `oasis update` command.
func newUpdateCmd() *cobra.Command {
	var targetVersion string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Pull the latest image and restart",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			running, _ := docker.ContainerRunning(cfg.ContainerName)
			if !running {
				fmt.Fprintln(os.Stderr, "Container is not running. Start it first with `oasis start`, then run `oasis update`.")
				os.Exit(1)
			}

			tag := "latest"
			if targetVersion != "" {
				tag = targetVersion
			}
			image := fmt.Sprintf("ghcr.io/prettysmartdev/oasis:%s", tag)

			if err := table.Spinner(fmt.Sprintf("Pulling %s...", image), func() error {
				return docker.PullImage(image, os.Stderr)
			}); err != nil {
				return fmt.Errorf("pulling image: %w", err)
			}

			// Extract port from the mgmt endpoint.
			port := extractPort(cfg.MgmtEndpoint, "04515")

			opts := docker.RunOptions{
				Name:       cfg.ContainerName,
				Image:      image,
				Port:       port,
				TsHostname: "oasis",
				MgmtPort:   port,
			}
			if err := docker.RemoveAndRerun(opts); err != nil {
				return fmt.Errorf("restarting container: %w", err)
			}

			// Update config with new version.
			cfg.LastKnownVersion = tag
			cfgPath := cfgFile
			if cfgPath == "" {
				cfgPath = config.DefaultPath()
			}
			_ = config.Save(cfgPath, cfg)

			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s — oasis is running.\n", tag)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&targetVersion, "version", "", "Target version tag (default: latest)")
	return cmd
}

// newLogsCmd returns the `oasis logs` command.
func newLogsCmd() *cobra.Command {
	var follow bool
	var lines int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream or print controller logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			return docker.Logs(cfg.ContainerName, follow, lines, os.Stdout)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "Number of lines to show from end of logs")
	return cmd
}

// loadConfig loads the CLI config from the configured path.
func loadConfig() (*config.Config, error) {
	path := cfgFile
	if path == "" {
		path = config.DefaultPath()
	}
	return config.Load(path)
}

// extractPort extracts the port from a URL like "http://127.0.0.1:04515".
func extractPort(endpoint, defaultPort string) string {
	idx := strings.LastIndex(endpoint, ":")
	if idx < 0 {
		return defaultPort
	}
	p := endpoint[idx+1:]
	// Strip any path component.
	if slash := strings.Index(p, "/"); slash >= 0 {
		p = p[:slash]
	}
	if p == "" {
		return defaultPort
	}
	return p
}
