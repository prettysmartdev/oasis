package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/prettysmartdev/oasis/internal/cli/client"
	"github.com/prettysmartdev/oasis/internal/cli/table"
	cliyaml "github.com/prettysmartdev/oasis/internal/cli/yaml"
	"github.com/spf13/cobra"
)

// agentCmd is the `oasis agent` subcommand group.
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage registered AI agents",
}

// agentRecord represents an agent from the management API.
type agentRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Prompt      string `json:"prompt"`
	Trigger     string `json:"trigger"`
	Schedule    string `json:"schedule"`
	OutputFmt   string `json:"outputFmt"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func init() {
	agentCmd.AddCommand(
		newAgentAddCmd(),
		newAgentNewCmd(),
		newAgentListCmd(),
		newAgentShowCmd(),
		newAgentRemoveCmd(),
		newAgentEnableCmd(),
		newAgentDisableCmd(),
		newAgentUpdateCmd(),
	)
}

func newAgentAddCmd() *cobra.Command {
	var (
		name, slug, prompt, trigger, schedule, outputFmt string
		description, icon, filePath                       string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a new agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			// File-based registration.
			if filePath != "" {
				flagsChanged := cmd.Flags().Changed("name") || cmd.Flags().Changed("slug") ||
					cmd.Flags().Changed("prompt") || cmd.Flags().Changed("trigger") ||
					cmd.Flags().Changed("schedule") || cmd.Flags().Changed("output-fmt") ||
					cmd.Flags().Changed("description") || cmd.Flags().Changed("icon")
				if flagsChanged {
					fmt.Fprintln(os.Stderr, "Flags ignored when -f is provided")
				}
				def, err := cliyaml.ParseAgentFile(filePath)
				if err != nil {
					fmt.Fprintln(os.Stderr, err.Error())
					os.Exit(1)
				}
				body := map[string]interface{}{
					"name":        def.Name,
					"slug":        def.Slug,
					"description": def.Description,
					"icon":        def.Icon,
					"prompt":      def.Prompt,
					"trigger":     def.Trigger,
					"schedule":    def.Schedule,
					"outputFmt":   def.OutputFmt,
					"enabled":     true,
				}
				var result agentRecord
				if err := newClient().Post("/api/v1/agents", body, &result); err != nil {
					if apiErr, ok := err.(*client.APIError); ok {
						if apiErr.HTTPStatus == 409 {
							fmt.Fprintf(os.Stderr, "A slug named %q already exists — choose a different slug.\n", def.Slug)
							os.Exit(1)
						}
						fmt.Fprintln(os.Stderr, apiErr.Message)
						os.Exit(1)
					}
					return err
				}
				if !quiet {
					fmt.Fprintf(cmd.OutOrStdout(), "Agent %q registered.\n", def.Name)
				}
				return nil
			}

			// Flag-based registration.
			if name == "" {
				fmt.Fprintln(os.Stderr, "--name is required")
				os.Exit(2)
			}
			if slug == "" {
				fmt.Fprintln(os.Stderr, "--slug is required")
				os.Exit(2)
			}
			if prompt == "" {
				fmt.Fprintln(os.Stderr, "--prompt is required")
				os.Exit(2)
			}
			if trigger == "" {
				fmt.Fprintln(os.Stderr, "--trigger is required")
				os.Exit(2)
			}
			if trigger == "schedule" && schedule == "" {
				fmt.Fprintln(os.Stderr, "--schedule is required when --trigger=schedule")
				os.Exit(2)
			}
			if outputFmt == "" {
				outputFmt = "markdown"
			}

			body := map[string]interface{}{
				"name":        name,
				"slug":        slug,
				"description": description,
				"icon":        icon,
				"prompt":      prompt,
				"trigger":     trigger,
				"schedule":    schedule,
				"outputFmt":   outputFmt,
				"enabled":     true,
			}

			var result agentRecord
			if err := newClient().Post("/api/v1/agents", body, &result); err != nil {
				if apiErr, ok := err.(*client.APIError); ok {
					if apiErr.HTTPStatus == 409 {
						fmt.Fprintf(os.Stderr, "A slug named %q already exists — choose a different slug.\n", slug)
						os.Exit(1)
					}
					fmt.Fprintln(os.Stderr, apiErr.Message)
					os.Exit(1)
				}
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent %q registered.\n", name)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "YAML definition file (overrides other flags)")
	cmd.Flags().StringVar(&name, "name", "", "Agent name")
	cmd.Flags().StringVar(&slug, "slug", "", "URL slug ([a-z0-9-]+)")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Agent prompt")
	cmd.Flags().StringVar(&trigger, "trigger", "", "Trigger type: tap, schedule, or webhook")
	cmd.Flags().StringVar(&schedule, "schedule", "", "Cron expression (required when --trigger=schedule)")
	cmd.Flags().StringVar(&outputFmt, "output-fmt", "", "Output format: markdown, html, or plaintext (default: markdown)")
	cmd.Flags().StringVar(&description, "description", "", "Agent description")
	cmd.Flags().StringVar(&icon, "icon", "", "Agent icon URL or emoji")

	return cmd
}

func newAgentNewCmd() *cobra.Command {
	var outputPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Generate an agent YAML definition template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			slug := sanitizeName(name)

			path := outputPath
			if path == "" {
				path = "./oasis-agent-" + slug + ".yaml"
			}

			if !force {
				if _, err := os.Stat(path); err == nil {
					fmt.Fprintf(os.Stderr, "File %s already exists. Use --force to overwrite.\n", path)
					os.Exit(1)
				}
			}

			content := fmt.Sprintf(`# OaSis agent definition — fill in the fields and run: oasis agent add -f ./oasis-agent.yaml
name: "%s"
slug: "%s"
description: ""
icon: ""
prompt: ""
trigger: "tap"
schedule: ""
outputFmt: "markdown"
`, name, slug)

			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent template written to %s\n", path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Output file path (default: ./oasis-agent-<slug>.yaml)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing file")

	return cmd
}

func newAgentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result struct {
				Items []agentRecord `json:"items"`
				Total int           `json:"total"`
			}
			if err := newClient().Get("/api/v1/agents", &result); err != nil {
				return err
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result.Items)
			}

			if len(result.Items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No agents registered yet. Use `oasis agent add` to register one.")
				return nil
			}

			headers := []string{"NAME", "SLUG", "TRIGGER", "OUTPUT FMT", "STATUS"}
			rows := make([][]string, len(result.Items))
			for i, a := range result.Items {
				status := "disabled"
				if a.Enabled {
					status = "enabled"
				}
				rows[i] = []string{a.Name, a.Slug, a.Trigger, a.OutputFmt, status}
			}
			table.PrintTable(headers, rows, cmd.OutOrStdout())
			return nil
		},
	}
}

func newAgentShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show details for a single agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			var a agentRecord
			if err := newClient().Get("/api/v1/agents/"+slug, &a); err != nil {
				if apiErr, ok := err.(*client.APIError); ok && apiErr.HTTPStatus == 404 {
					fmt.Fprintf(os.Stderr, "No agent found with slug %q.\n", slug)
					os.Exit(1)
				}
				return err
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(a)
			}

			status := "disabled"
			if a.Enabled {
				status = "enabled"
			}

			table.PrintKV([]table.KVPair{
				{Key: "ID", Value: a.ID},
				{Key: "Name", Value: a.Name},
				{Key: "Slug", Value: a.Slug},
				{Key: "Description", Value: a.Description},
				{Key: "Icon", Value: a.Icon},
				{Key: "Prompt", Value: a.Prompt},
				{Key: "Trigger", Value: a.Trigger},
				{Key: "Schedule", Value: a.Schedule},
				{Key: "Output Format", Value: a.OutputFmt},
				{Key: "Status", Value: status},
				{Key: "Created", Value: a.CreatedAt},
				{Key: "Updated", Value: a.UpdatedAt},
			}, cmd.OutOrStdout())
			return nil
		},
	}
}

func newAgentRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <slug>",
		Short: "Unregister and remove an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if err := newClient().Delete("/api/v1/agents/" + slug); err != nil {
				if apiErr, ok := err.(*client.APIError); ok && apiErr.HTTPStatus == 404 {
					fmt.Fprintf(os.Stderr, "No agent found with slug %q.\n", slug)
					os.Exit(1)
				}
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent %q removed.\n", slug)
			}
			return nil
		},
	}
}

func newAgentEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <slug>",
		Short: "Enable a disabled agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if err := newClient().Post("/api/v1/agents/"+slug+"/enable", nil, nil); err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent %q enabled.\n", slug)
			}
			return nil
		},
	}
}

func newAgentDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <slug>",
		Short: "Disable an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if err := newClient().Post("/api/v1/agents/"+slug+"/disable", nil, nil); err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent %q disabled.\n", slug)
			}
			return nil
		},
	}
}

func newAgentUpdateCmd() *cobra.Command {
	var name, prompt, schedule, outputFmt, description, icon, trigger string

	cmd := &cobra.Command{
		Use:   "update <slug>",
		Short: "Update agent fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]

			body := make(map[string]interface{})
			if cmd.Flags().Changed("name") {
				body["name"] = name
			}
			if cmd.Flags().Changed("prompt") {
				body["prompt"] = prompt
			}
			if cmd.Flags().Changed("schedule") {
				body["schedule"] = schedule
			}
			if cmd.Flags().Changed("output-fmt") {
				body["outputFmt"] = outputFmt
			}
			if cmd.Flags().Changed("description") {
				body["description"] = description
			}
			if cmd.Flags().Changed("icon") {
				body["icon"] = icon
			}
			if cmd.Flags().Changed("trigger") {
				body["trigger"] = trigger
			}

			if len(body) == 0 {
				fmt.Fprintln(os.Stderr, "Nothing to update — provide at least one flag.")
				os.Exit(2)
			}

			if err := newClient().Patch("/api/v1/agents/"+slug, body, nil); err != nil {
				if apiErr, ok := err.(*client.APIError); ok && apiErr.HTTPStatus == 404 {
					fmt.Fprintf(os.Stderr, "No agent found with slug %q.\n", slug)
					os.Exit(1)
				}
				return err
			}

			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Agent %q updated.\n", slug)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New display name")
	cmd.Flags().StringVar(&prompt, "prompt", "", "New prompt")
	cmd.Flags().StringVar(&schedule, "schedule", "", "New cron schedule")
	cmd.Flags().StringVar(&outputFmt, "output-fmt", "", "New output format (markdown, html, plaintext)")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	cmd.Flags().StringVar(&icon, "icon", "", "New icon URL or emoji")
	cmd.Flags().StringVar(&trigger, "trigger", "", "New trigger type (tap, schedule, webhook)")

	return cmd
}

