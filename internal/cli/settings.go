package cli

import "github.com/spf13/cobra"

// settingsCmd is the `oasis settings` subcommand group.
var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "View and modify oasis settings",
}

func init() {
	settingsCmd.AddCommand(
		newStubCmd("get", "Print current settings (or a single key's value)"),
		newStubCmd("set", "Update a settings value"),
	)
}
