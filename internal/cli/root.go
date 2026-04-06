// Package cli implements the oasis command-line interface.
package cli

import (
	"os"

	"github.com/prettysmartdev/oasis/internal/cli/client"
	"github.com/prettysmartdev/oasis/internal/cli/config"
	"github.com/spf13/cobra"
)

// Global flag values — bound by cobra and read in command implementations.
var (
	cfgFile string
	jsonOut bool
	quiet   bool

	// cliVersion holds the embedded version string set by Execute.
	cliVersion string
)

// rootCmd is the base command for the oasis CLI.
var rootCmd = &cobra.Command{
	Use:   "oasis",
	Short: "oasis — manage your self-hosted app dashboard",
	Long: `oasis is a self-hosted dashboard for your vibe-coded apps and AI agents,
exposed exclusively over your Tailscale network.`,
	SilenceUsage: true,
}

// Execute runs the root command with the embedded version string.
func Execute(version string) {
	cliVersion = version
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newClient creates an API client using the current config and CLI version.
func newClient() *client.Client {
	path := cfgFile
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, _ := config.Load(path) // ignore error; use defaults if not found
	return client.New(cfg.MgmtEndpoint, rootCmd.Version)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "CLI config file (default: ~/.oasis/config.json)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-error output")

	rootCmd.AddCommand(appCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newDbCmd())
}
