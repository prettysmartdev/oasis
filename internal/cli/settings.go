package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/prettysmartdev/oasis/internal/cli/client"
	"github.com/prettysmartdev/oasis/internal/cli/config"
	"github.com/prettysmartdev/oasis/internal/cli/table"
	"github.com/spf13/cobra"
)

// settingsCmd is the `oasis settings` subcommand group.
var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "View and modify oasis settings",
}

// validSettingsKeys lists all supported setting key names.
var validSettingsKeys = []string{"tailscaleHostname", "mgmtPort", "theme"}

func init() {
	settingsCmd.AddCommand(
		newSettingsGetCmd(),
		newSettingsSetCmd(),
	)
}

func isValidSettingKey(key string) bool {
	for _, k := range validSettingsKeys {
		if k == key {
			return true
		}
	}
	return false
}

// settingsResponse mirrors the /api/v1/settings response shape.
type settingsResponse struct {
	TailscaleHostname string `json:"tailscaleHostname"`
	MgmtPort          string `json:"mgmtPort"`
	Theme             string `json:"theme"`
}

func newSettingsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Print current settings (or a single key's value)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var s settingsResponse
			if err := newClient().Get("/api/v1/settings", &s); err != nil {
				return err
			}

			if len(args) == 1 {
				key := args[0]
				if !isValidSettingKey(key) {
					fmt.Fprintf(os.Stderr, "Unknown setting %q. Valid keys: tailscaleHostname, mgmtPort, theme.\n", key)
					os.Exit(2)
				}
				switch key {
				case "tailscaleHostname":
					fmt.Fprintln(cmd.OutOrStdout(), s.TailscaleHostname)
				case "mgmtPort":
					fmt.Fprintln(cmd.OutOrStdout(), s.MgmtPort)
				case "theme":
					fmt.Fprintln(cmd.OutOrStdout(), s.Theme)
				}
				return nil
			}

			table.PrintKV([]table.KVPair{
				{Key: "tailscaleHostname", Value: s.TailscaleHostname},
				{Key: "mgmtPort", Value: s.MgmtPort},
				{Key: "theme", Value: s.Theme},
			}, cmd.OutOrStdout())
			return nil
		},
	}
}

func newSettingsSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update a settings value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			if !isValidSettingKey(key) {
				fmt.Fprintf(os.Stderr, "Unknown setting %q. Valid keys: tailscaleHostname, mgmtPort, theme.\n", key)
				os.Exit(2)
			}

			// Validate mgmtPort range.
			if key == "mgmtPort" {
				port, err := strconv.Atoi(value)
				if err != nil || port < 1 || port > 65535 {
					fmt.Fprintf(os.Stderr, "mgmtPort must be a number between 1 and 65535, got %q.\n", value)
					os.Exit(2)
				}
			}

			body := map[string]interface{}{key: value}
			if err := newClient().Patch("/api/v1/settings", body, nil); err != nil {
				if apiErr, ok := err.(*client.APIError); ok {
					fmt.Fprintln(os.Stderr, apiErr.Message)
					os.Exit(1)
				}
				return err
			}

			// If mgmtPort changed, update config so subsequent commands use the new port.
			if key == "mgmtPort" {
				cfgPath := cfgFile
				if cfgPath == "" {
					cfgPath = config.DefaultPath()
				}
				if cfg, err := loadConfig(); err == nil {
					newEndpoint := fmt.Sprintf("http://127.0.0.1:%s", value)
					if cfg.MgmtEndpoint != newEndpoint {
						cfg.MgmtEndpoint = newEndpoint
						_ = config.Save(cfgPath, cfg)
					}
				}
			}

			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Setting %q updated.\n", key)
			}
			return nil
		},
	}
}
