# Operations

## Installing and running

Installation:
- Install the oasis CLI binary (the only thing needed on the host machine):
  - macOS/Linux via Homebrew: `brew install [owner]/tap/oasis`
  - Manual: download the appropriate binary from the GitHub releases page, chmod +x, and place in /usr/local/bin/oasis
- The Docker image (ghcr.io/[owner]/oasis) is pulled automatically by the CLI during `oasis init`; users do not need to pull it manually
- Prerequisites: Docker must be running on the host; Tailscale account with access to generate auth keys

Setup and run:
- Run `oasis init` to perform first-time setup:
  1. Prompts for a Tailscale auth key (obtain from the Tailscale admin console under Settings > Keys; use a reusable, pre-authorized key)
  2. Prompts for a Tailscale hostname for the oasis node (default: oasis — will be accessible as oasis.[tailnet-name].ts.net)
  3. Optionally prompts for the management API port (default: 04515)
  4. Pulls the Docker image, creates named volumes, and starts the container
  5. Polls until the controller is reachable and Tailscale is connected, then prints the dashboard URL
- Access the dashboard by visiting https://oasis.[tailnet-name].ts.net from any device on the tailnet
- Register apps: `oasis app add --name "My App" --url http://localhost:3000 --slug myapp`
- Check status: `oasis status`

Environment variables (passed to the Docker container):
- TS_AUTHKEY: Tailscale auth key — required on first start; not needed on subsequent starts if the tsnet state volume is intact
- OASIS_MGMT_PORT: management API port (default: 04515)
- OASIS_HOSTNAME: Tailscale hostname for the oasis node (default: oasis)
- OASIS_DB_PATH: path to the SQLite database file inside the container (default: /data/db/oasis.db)
- OASIS_TS_STATE_DIR: path to the Tailscale tsnet state directory inside the container (default: /data/ts-state)
- OASIS_LOG_LEVEL: log verbosity — "info" (default), "debug", "warn", "error"

Secrets:
- TS_AUTHKEY is the only secret; treat it accordingly — do not commit it to version control or include it in shell history
- Pass it via `oasis init` (which handles the docker run invocation) or via a secrets manager that injects environment variables
- After initial authentication the auth key is no longer needed; the tsnet state volume provides persistent identity
- Rotate the auth key via the Tailscale admin console if needed; the container does not need to be restarted unless the node is removed from the tailnet

## Ongoing operations

Version upgrades/downgrades:
- Upgrade to the latest release: `oasis update` — pulls the latest tagged image and restarts the container with volumes intact
- Upgrade to a specific version: `oasis update --version v1.2.3`
- Downgrade by specifying an older version tag: `oasis update --version v1.1.0`
- CLI binary upgrades are independent: `brew upgrade oasis` or download a new binary from GitHub releases
- The controller logs a warning on startup if the CLI version is more than one minor version behind the container version

Database migrations:
- The controller applies SQLite schema migrations automatically at startup using embedded migration files (go:embed)
- Migrations are additive only — no destructive schema changes (no DROP COLUMN, no data-loss alterations)
- Before each migration run the controller writes a backup copy of the database to /data/db/oasis.db.bak
- Manual backup: `oasis db backup --output ./oasis-backup-$(date +%F).db` — copies the live database via the management API
- To restore from backup: stop the container, replace /data/db/oasis.db via `docker cp`, restart the container
