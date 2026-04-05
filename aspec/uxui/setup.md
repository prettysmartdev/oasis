# Setup

## User Installation

Download:
- Via Homebrew (recommended for macOS and Linux):
  `brew install [owner]/tap/oasis`
- Via GitHub releases (manual):
  - macOS (Apple Silicon): oasis_darwin_arm64.tar.gz
  - macOS (Intel): oasis_darwin_amd64.tar.gz
  - Linux (x86_64): oasis_linux_amd64.tar.gz
  - Linux (ARM64, e.g. Raspberry Pi): oasis_linux_arm64.tar.gz
  - Extract the tarball, chmod +x oasis, move to /usr/local/bin/oasis
- The Docker image (ghcr.io/[owner]/oasis) is pulled automatically by `oasis init` — users do not need to interact with Docker directly

Initial configuration:
- Run `oasis init` after installing the CLI binary — this is the only required setup step
- The wizard prompts for:
  1. Tailscale auth key — obtain from the Tailscale admin console under Settings > Keys; the CLI prints the URL; use a reusable, pre-authorized key so the container can reconnect after restarts without a new key
  2. Tailscale hostname for the oasis node (default: "oasis") — the dashboard will be at https://[hostname].[tailnet-name].ts.net
  3. Management API port (default: 04515) — change only if 04515 is already in use on the host
- `oasis init` creates ~/.oasis/config.json, starts the container with the correct volume mounts and port bindings, and polls until the controller confirms Tailscale is connected
- On success, the CLI prints: "Your oasis is ready at https://[hostname].[tailnet-name].ts.net"

Superuser access:
- The oasis CLI requires Docker to be available; the user must be in the docker group (Linux) or have Docker Desktop installed (macOS/Windows)
- No root/sudo is required to run the CLI itself
- All processes inside the container run as non-root (uid 1000); no privileged container mode is needed
- On macOS with Docker Desktop, no additional permission configuration is needed beyond the Docker Desktop installation
- If the user is not in the docker group on Linux, they can either add themselves (`sudo usermod -aG docker $USER`, then log out/in) or prefix CLI commands with sudo — the CLI does not require sudo directly but the underlying docker commands must be authorized
