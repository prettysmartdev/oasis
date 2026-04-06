package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newDbCmd returns the `oasis db` parent command with the `backup` subcommand.
func newDbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database management",
	}
	cmd.AddCommand(newDbBackupCmd())
	return cmd
}

func newDbBackupCmd() *cobra.Command {
	var outputPath string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Copy the SQLite database out of the container",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			if outputPath == "" {
				ts := time.Now().UTC().Format(time.RFC3339)
				// Replace colons for filename safety.
				ts = strings.ReplaceAll(ts, ":", "-")
				outputPath = fmt.Sprintf("./oasis-backup-%s.db", ts)
			}

			if dbPath == "" {
				dbPath = "/data/db/oasis.db"
			}

			dockerPath, err := exec.LookPath("docker")
			if err != nil {
				fmt.Fprintln(os.Stderr, "docker is not installed or not in PATH — install it at https://docs.docker.com/get-docker/")
				os.Exit(1)
			}

			src := fmt.Sprintf("%s:%s", cfg.ContainerName, dbPath)
			c := exec.Command(dockerPath, "cp", src, outputPath)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("docker cp failed: %w", err)
			}

			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Database backed up to %s.\n", outputPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Output file path (default: ./oasis-backup-<timestamp>.db)")
	cmd.Flags().StringVar(&dbPath, "db-path", "", "Path to database inside container (default: /data/db/oasis.db)")

	return cmd
}
