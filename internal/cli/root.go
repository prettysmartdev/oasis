// Package cli implements the oasis command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Global flag values — bound by cobra and read in command implementations (future work items).
var (
	cfgFile string
	jsonOut bool
	quiet   bool
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
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "CLI config file (default: ~/.oasis/config.json)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-error output")

	rootCmd.AddCommand(appCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(newStubCmd("init", "Interactive first-time setup"))
	rootCmd.AddCommand(newStubCmd("start", "Start the oasis container"))
	rootCmd.AddCommand(newStubCmd("stop", "Stop the oasis container"))
	rootCmd.AddCommand(newStubCmd("restart", "Restart the oasis container"))
	rootCmd.AddCommand(newStubCmd("status", "Show controller status"))
	rootCmd.AddCommand(newStubCmd("update", "Pull the latest image and restart"))
	rootCmd.AddCommand(newStubCmd("logs", "Stream or print controller logs"))
	rootCmd.AddCommand(newStubCmd("db", "Database management"))
}

// newStubCmd creates a placeholder command that prints "not yet implemented".
func newStubCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "not yet implemented")
			return nil
		},
	}
}
