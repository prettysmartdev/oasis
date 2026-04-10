// Package docker provides helpers for managing the oasis container via the Docker CLI.
package docker

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// RunOptions holds the parameters for starting a new oasis container.
type RunOptions struct {
	Name       string
	Image      string
	Port       string
	TsAuthKey  string
	TsHostname string
	MgmtPort   string
	// Claude auth mount fields. When MountClaude is true, ClaudeJSONPath and
	// ClaudeDirPath are bind-mounted read-only into the container.
	MountClaude    bool
	ClaudeJSONPath string // absolute host path to ~/.claude.json
	ClaudeDirPath  string // absolute host path to ~/.claude/
}

// lookupDocker returns the path to the docker binary or an error if not found.
func lookupDocker() (string, error) {
	path, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("docker is not installed or not in PATH — install it at https://docs.docker.com/get-docker/")
	}
	return path, nil
}

// run executes a docker command with the given arguments, returning its combined output.
func run(args ...string) (string, error) {
	docker, err := lookupDocker()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(docker, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// runStream executes a docker command, streaming stdout/stderr to out.
func runStream(out io.Writer, args ...string) error {
	docker, err := lookupDocker()
	if err != nil {
		return err
	}
	cmd := exec.Command(docker, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

// PullImage runs `docker pull <image>`, streaming output to out.
func PullImage(image string, out io.Writer) error {
	return runStream(out, "pull", image)
}

// RunContainer starts the oasis container in detached mode with the given options.
func RunContainer(opts RunOptions) error {
	port := opts.Port
	if port == "" {
		port = "04515"
	}
	mgmtPort := opts.MgmtPort
	if mgmtPort == "" {
		mgmtPort = "04515"
	}

	dockerArgs := []string{
		"run", "-d",
		"--name", opts.Name,
		"--restart", "unless-stopped",
		"-v", "oasis-db:/data/db",
		"-v", "oasis-ts-state:/data/ts-state",
		"-v", "oasis-agent-runs:/data/agent-runs",
		"-p", fmt.Sprintf("127.0.0.1:%s:%s", port, mgmtPort),
		"-e", fmt.Sprintf("TS_AUTHKEY=%s", opts.TsAuthKey),
		"-e", fmt.Sprintf("OASIS_HOSTNAME=%s", opts.TsHostname),
		"-e", fmt.Sprintf("OASIS_MGMT_PORT=%s", mgmtPort),
	}

	// Add claude auth mounts if both paths exist on the host.
	if opts.MountClaude && opts.ClaudeJSONPath != "" && opts.ClaudeDirPath != "" {
		dockerArgs = append(dockerArgs,
			"-v", opts.ClaudeJSONPath+":/root/.claude.json:ro",
			"-v", opts.ClaudeDirPath+":/root/.claude/:ro",
		)
	}

	dockerArgs = append(dockerArgs, opts.Image)

	_, err := run(dockerArgs...)
	return err
}

// StartContainer runs `docker start <name>`.
func StartContainer(name string) error {
	_, err := run("start", name)
	return err
}

// StopContainer runs `docker stop <name>`.
func StopContainer(name string) error {
	_, err := run("stop", name)
	return err
}

// RestartContainer runs `docker restart <name>`.
func RestartContainer(name string) error {
	_, err := run("restart", name)
	return err
}

// RemoveContainer runs `docker rm <name>`.
func RemoveContainer(name string) error {
	_, err := run("rm", name)
	return err
}

// ContainerExists returns true if the named container exists (running or stopped).
func ContainerExists(name string) (bool, error) {
	_, err := lookupDocker()
	if err != nil {
		return false, err
	}
	out, err := run("inspect", "--format", "{{.Name}}", name)
	if err != nil {
		if strings.Contains(out, "No such container") || strings.Contains(err.Error(), "No such container") {
			return false, nil
		}
		// docker inspect exits non-zero for missing containers; treat as not-existing.
		return false, nil
	}
	return out != "", nil
}

// ContainerRunning returns true if the named container exists and is running.
func ContainerRunning(name string) (bool, error) {
	out, err := run("inspect", "--format", "{{.State.Running}}", name)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(out) == "true", nil
}

// Logs streams or prints container logs to out.
// If follow is true, `--follow` is passed to docker logs.
// tail is the number of lines to show from the end (0 means all).
func Logs(name string, follow bool, tail int, out io.Writer) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, name)
	return runStream(out, args...)
}

// RemoveAndRerun stops and removes an existing container, then starts a fresh one.
// Used by `oasis update` to guarantee the new image is applied.
func RemoveAndRerun(opts RunOptions) error {
	// Best-effort stop; ignore error if already stopped.
	_ = StopContainer(opts.Name)
	if err := RemoveContainer(opts.Name); err != nil {
		return fmt.Errorf("failed to remove existing container: %w", err)
	}
	return RunContainer(opts)
}
