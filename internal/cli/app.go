package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/prettysmartdev/oasis/internal/cli/client"
	"github.com/prettysmartdev/oasis/internal/cli/table"
	"github.com/spf13/cobra"
)

// appCmd is the `oasis app` subcommand group.
var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage registered apps",
}

var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func init() {
	appCmd.AddCommand(
		newAppAddCmd(),
		newAppListCmd(),
		newAppShowCmd(),
		newAppRemoveCmd(),
		newAppEnableCmd(),
		newAppDisableCmd(),
		newAppUpdateCmd(),
	)
}

// appRecord represents an app from the management API.
type appRecord struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	UpstreamURL string   `json:"upstreamURL"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Tags        []string `json:"tags"`
	Enabled     bool     `json:"enabled"`
	Health      string   `json:"health"`
}

func newAppAddCmd() *cobra.Command {
	var name, upstreamURL, slug, description, icon, tags string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a new app",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Client-side validation.
			if !slugPattern.MatchString(slug) {
				fmt.Fprintln(os.Stderr, `slug must match [a-z0-9-]+ (e.g. my-app)`)
				os.Exit(2)
			}
			if !isValidURL(upstreamURL) {
				fmt.Fprintln(os.Stderr, "URL must start with http:// or https://")
				os.Exit(2)
			}

			var tagList []string
			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tagList = append(tagList, t)
					}
				}
			}

			body := map[string]interface{}{
				"name":        name,
				"slug":        slug,
				"upstreamURL": upstreamURL,
				"description": description,
				"icon":        icon,
				"tags":        tagList,
				"enabled":     true,
			}

			var result appRecord
			err := newClient().Post("/api/v1/apps", body, &result)
			if err != nil {
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
				fmt.Fprintf(cmd.OutOrStdout(), "App %q registered at /%s\n", name, slug)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "App display name (required)")
	cmd.Flags().StringVar(&upstreamURL, "url", "", "Upstream URL (required, must start with http:// or https://)")
	cmd.Flags().StringVar(&slug, "slug", "", "URL slug (required, [a-z0-9-]+)")
	cmd.Flags().StringVar(&description, "description", "", "App description")
	cmd.Flags().StringVar(&icon, "icon", "", "App icon URL or emoji")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("slug")

	return cmd
}

func newAppListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result struct {
				Items []appRecord `json:"items"`
				Total int         `json:"total"`
			}
			if err := newClient().Get("/api/v1/apps", &result); err != nil {
				return err
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result.Items)
			}

			if len(result.Items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No apps registered yet. Use `oasis app add` to register one.")
				return nil
			}

			headers := []string{"NAME", "SLUG", "URL", "STATUS", "HEALTH"}
			rows := make([][]string, len(result.Items))
			for i, a := range result.Items {
				status := "disabled"
				if a.Enabled {
					status = "enabled"
				}
				health := a.Health
				if health == "" {
					health = "unknown"
				}
				rows[i] = []string{a.Name, a.Slug, a.UpstreamURL, status, health}
			}
			table.PrintTable(headers, rows, cmd.OutOrStdout())
			return nil
		},
	}
}

func newAppShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show details for a single app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			var a appRecord
			if err := newClient().Get("/api/v1/apps/"+slug, &a); err != nil {
				if apiErr, ok := err.(*client.APIError); ok && apiErr.HTTPStatus == 404 {
					fmt.Fprintf(os.Stderr, "No app found with slug %q.\n", slug)
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
			health := a.Health
			if health == "" {
				health = "unknown"
			}

			table.PrintKV([]table.KVPair{
				{Key: "ID", Value: a.ID},
				{Key: "Name", Value: a.Name},
				{Key: "Slug", Value: a.Slug},
				{Key: "URL", Value: a.UpstreamURL},
				{Key: "Description", Value: a.Description},
				{Key: "Icon", Value: a.Icon},
				{Key: "Tags", Value: strings.Join(a.Tags, ", ")},
				{Key: "Status", Value: status},
				{Key: "Health", Value: health},
			}, cmd.OutOrStdout())
			return nil
		},
	}
}

func newAppRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <slug>",
		Short: "Unregister and remove an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if err := newClient().Delete("/api/v1/apps/" + slug); err != nil {
				if apiErr, ok := err.(*client.APIError); ok && apiErr.HTTPStatus == 404 {
					fmt.Fprintf(os.Stderr, "No app found with slug %q.\n", slug)
					os.Exit(1)
				}
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "App %q removed.\n", slug)
			}
			return nil
		},
	}
}

func newAppEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <slug>",
		Short: "Enable a disabled app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if err := newClient().Post("/api/v1/apps/"+slug+"/enable", nil, nil); err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "App %q enabled.\n", slug)
			}
			return nil
		},
	}
}

func newAppDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <slug>",
		Short: "Disable an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if err := newClient().Post("/api/v1/apps/"+slug+"/disable", nil, nil); err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "App %q disabled.\n", slug)
			}
			return nil
		},
	}
}

func newAppUpdateCmd() *cobra.Command {
	var name, upstreamURL, description, icon, tags string

	cmd := &cobra.Command{
		Use:   "update <slug>",
		Short: "Update app fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]

			body := make(map[string]interface{})
			if cmd.Flags().Changed("name") {
				body["name"] = name
			}
			if cmd.Flags().Changed("url") {
				body["upstreamURL"] = upstreamURL
			}
			if cmd.Flags().Changed("description") {
				body["description"] = description
			}
			if cmd.Flags().Changed("icon") {
				body["icon"] = icon
			}
			if cmd.Flags().Changed("tags") {
				var tagList []string
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tagList = append(tagList, t)
					}
				}
				body["tags"] = tagList
			}

			if len(body) == 0 {
				fmt.Fprintln(os.Stderr, "Nothing to update — provide at least one flag.")
				os.Exit(2)
			}

			if err := newClient().Patch("/api/v1/apps/"+slug, body, nil); err != nil {
				if apiErr, ok := err.(*client.APIError); ok && apiErr.HTTPStatus == 404 {
					fmt.Fprintf(os.Stderr, "No app found with slug %q.\n", slug)
					os.Exit(1)
				}
				return err
			}

			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "App %q updated.\n", slug)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New display name")
	cmd.Flags().StringVar(&upstreamURL, "url", "", "New upstream URL")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	cmd.Flags().StringVar(&icon, "icon", "", "New icon URL or emoji")
	cmd.Flags().StringVar(&tags, "tags", "", "New comma-separated tags")

	return cmd
}
