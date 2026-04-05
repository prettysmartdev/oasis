package cli

import "github.com/spf13/cobra"

// appCmd is the `oasis app` subcommand group.
var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage registered apps",
}

func init() {
	appCmd.AddCommand(
		newStubCmd("add", "Register a new app"),
		newStubCmd("list", "List all registered apps"),
		newStubCmd("show", "Show details for a single app"),
		newStubCmd("remove", "Unregister and remove an app"),
		newStubCmd("enable", "Enable a disabled app"),
		newStubCmd("disable", "Disable an app"),
		newStubCmd("update", "Update app fields"),
	)
}
