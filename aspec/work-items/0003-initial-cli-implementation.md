# Work Item: Feature

Title: Initial CLI Implementation
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary

Implement all stub CLI commands in `internal/cli/` so the `oasis` binary is fully functional end-to-end against a running controller. The CLI manages the oasis Docker container, calls the management API for app and settings operations, and guides users through first-time setup with a friendly interactive wizard. The webapp is explicitly out of scope; the controller requires no changes unless a bug surfaces during integration testing.

---

## User Stories

### User Story 1:
As a: Owner / Admin

I want to:
Run `oasis init` and answer a few short prompts (Tailscale auth key, hostname, port), then have the CLI pull the Docker image, start the container, and wait until my tailnet node is ready.

So I can:
Go from zero to a working oasis instance in under two minutes without touching Docker commands, config files, or the Tailscale admin panel beyond copying an auth key.

### User Story 2:
As a: Owner / Admin

I want to:
Register, list, inspect, update, enable/disable, and remove apps using `oasis app` subcommands with human-readable table output by default and clean JSON when I pass `--json`.

So I can:
Manage my app registry from the terminal without remembering API paths or writing curl commands, and pipe structured output into scripts when needed.

### User Story 3:
As a: Owner / Admin

I want to:
Check `oasis status` at any time and see a clear summary of whether the container is running, whether the tailnet connection is live, what NGINX is doing, and how many apps are registered.

So I can:
Quickly diagnose problems — a downed container, a dropped tailnet connection, or a crashed NGINX process — without shelling into the container or reading raw logs.

---

## Implementation Details

### New packages to create

#### `internal/cli/config`

- Defines `Config` struct: `MgmtEndpoint string`, `ContainerName string`, `LastKnownVersion string`.
- `Load(path string) (*Config, error)` — reads and JSON-decodes the config file; returns defaults if the file does not exist (does not error on missing file).
- `Save(path string, cfg *Config) error` — atomically writes the config file (temp file + rename).
- `DefaultPath() string` — returns `~/.oasis/config.json` (expand `$HOME`).
- Defaults: `MgmtEndpoint = "http://127.0.0.1:04515"`, `ContainerName = "oasis"`.

#### `internal/cli/client`

- `Client` wraps `*http.Client` and holds `baseURL string` and `cliVersion string`.
- `New(baseURL, cliVersion string) *Client`.
- All request methods (`Get`, `Post`, `Patch`, `Delete`) set `Content-Type: application/json` and `X-Oasis-CLI-Version` on every request.
- `client.Get(path string, out interface{}) error` — decodes the response body into `out`; on non-2xx returns a typed `APIError{Code, Message, HTTPStatus}`.
- `client.Post`, `Patch`, `Delete` follow the same pattern.
- Timeout: 10 seconds (configurable via an option, for long-polling during `oasis init`).
- `APIError.Error()` returns a clean human-readable string; callers never need to inspect raw HTTP status codes.

#### `internal/cli/docker`

- Shells out to the `docker` binary (found via `exec.LookPath`); returns a clear error if Docker is not installed.
- `PullImage(image string, out io.Writer) error` — runs `docker pull <image>`, streams output to `out` when it is a TTY (spinner otherwise).
- `RunContainer(opts RunOptions) error` — runs `docker run -d` with the correct flags: name, volume mounts (`oasis-db:/data/db`, `oasis-ts-state:/data/ts-state`), port binding (`127.0.0.1:<port>:04515`), env vars (`TS_AUTHKEY`, `OASIS_HOSTNAME`, `OASIS_MGMT_PORT`), restart policy `unless-stopped`, and image tag.
- `StartContainer(name string) error`, `StopContainer(name string) error`, `RestartContainer(name string) error` — wrap `docker start/stop/restart`.
- `ContainerExists(name string) (bool, error)` — `docker inspect --format '{{.Name}}' <name>` to probe existence without an error on missing.
- `ContainerRunning(name string) (bool, error)` — checks `docker inspect --format '{{.State.Running}}'`.
- `Logs(name string, follow bool, tail int, out io.Writer) error` — runs `docker logs [--follow] [--tail N] <name>`, writes to `out`.
- `PullLatestTag(image string) (string, error)` — queries the registry for the `latest` digest; returns the resolved tag string for display.
- No Docker SDK dependency — shelling out keeps the binary small, avoids CGO, and relies only on the Docker CLI the user already has.

### Command implementations

#### `internal/cli/init.go` — `oasis init`

Interactive wizard; the only command that reads from stdin.

Steps:
1. Check that `docker` is available; print a friendly install hint if not.
2. If `~/.oasis/config.json` already exists and the container is running, print "oasis is already running" and offer `oasis status` as the next step; exit 0.
3. Prompt for:
   - Tailscale auth key (masked input; print the Tailscale admin URL as a hint).
   - Tailscale hostname (default: `oasis`).
   - Management port (default: `04515`; only ask if `--advanced` flag is set — hide this from casual users).
4. Pull `ghcr.io/[owner]/oasis:latest` with a progress spinner.
5. Call `docker.RunContainer` with the collected values.
6. Poll `GET /api/v1/status` every 2 seconds for up to 90 seconds, showing a spinner. Consider the node ready when `tailscaleConnected == true`.
7. Write `~/.oasis/config.json` with `mgmtEndpoint`, `containerName`, and `lastKnownVersion`.
8. Print success message: `Your oasis is ready at https://<hostname>.<tailnet>.ts.net` (use the `tailscaleHostname` from the status response).

The `--advanced` flag on `oasis init` unhides the port prompt. It is not listed in `--help` output to keep the default experience clean (use `cobra.Command.Hidden = false` but omit from the usage template, or just add the flag quietly — `--advanced` only affects prompt rendering).

#### `internal/cli/container.go` — `start`, `stop`, `restart`, `status`, `update`, `logs`

**`oasis start`**
- Load config; call `docker.StartContainer(cfg.ContainerName)`.
- If container does not exist, print "oasis is not initialised — run `oasis init` first".

**`oasis stop`**
- Load config; call `docker.StopContainer(cfg.ContainerName)`.

**`oasis restart`**
- Load config; call `docker.RestartContainer(cfg.ContainerName)`.

**`oasis status`**
- Load config; call `client.Get("/api/v1/status", &status)`.
- If the management API is unreachable, also check `docker.ContainerRunning` to distinguish "container stopped" from "controller not responding".
- Human output: a two-column table with labels and values (Tailscale, NGINX, apps registered, version).
- JSON output: the raw `Status` object from the API.

**`oasis update [--version v1.2.3]`**
- Resolve the target image tag (default: `latest`).
- Pull the image with a spinner.
- Call `docker.RestartContainer` after pull (docker will use the newly pulled image on restart only if the image reference resolves to the new digest — call `docker stop` + `docker rm` + `docker.RunContainer` with the new tag to guarantee the update is applied).
- On success, update `lastKnownVersion` in config.
- Print "Updated to vX.Y.Z — oasis is running."

**`oasis logs [--follow] [--lines N]`**
- Flags: `--follow` / `-f` (default false), `--lines` / `-n` (default 50).
- Calls `docker.Logs(cfg.ContainerName, follow, lines, os.Stdout)`.

#### `internal/cli/app.go` — `app` subcommands

Replace all stub commands with real implementations.

**`oasis app add`**

Flags: `--name` (required), `--url` (required), `--slug` (required), `--description`, `--icon`, `--tags` (comma-separated).

- Validate `--slug` client-side: must match `[a-z0-9-]+`; print a clear error with an example if invalid.
- POST `/api/v1/apps` with a JSON body; on `409` print "A slug named '<slug>' already exists — choose a different slug." On `400` show the API error message directly. On success, print a one-line confirmation: `App "<name>" registered at /<slug>`.

**`oasis app list [--json]`**

- GET `/api/v1/apps`; render a table: `NAME`, `SLUG`, `URL`, `STATUS`, `HEALTH` columns.
- When `--json`, print the raw items array.
- When no apps are registered, print "No apps registered yet. Use `oasis app add` to register one." instead of an empty table.

**`oasis app show <slug> [--json]`**

- GET `/api/v1/apps/<slug>`; render a vertical key/value table.
- `404` → "No app found with slug '<slug>'."

**`oasis app remove <slug>`**

- DELETE `/api/v1/apps/<slug>`.
- Print "App '<slug>' removed." on success.
- On `404`, print "No app found with slug '<slug>'."

**`oasis app enable <slug>` / `oasis app disable <slug>`**

- POST `/api/v1/apps/<slug>/enable` or `/disable`.
- Print "App '<slug>' enabled." / "App '<slug>' disabled."

**`oasis app update <slug>`**

Flags: `--name`, `--url`, `--description`, `--icon`, `--tags` (all optional).

- If no flags are provided, print "Nothing to update — provide at least one flag." and exit 2.
- PATCH `/api/v1/apps/<slug>` with only the fields that were explicitly set.
- Print "App '<slug>' updated."

#### `internal/cli/settings.go` — `settings` subcommands

**`oasis settings get [key]`**

- GET `/api/v1/settings`; with no argument, render all settings as a key/value table; with a key argument, print only that value (suitable for scripting).
- Supported keys: `tailscaleHostname`, `mgmtPort`, `theme`.
- Unknown key → print "Unknown setting '<key>'. Valid keys: tailscaleHostname, mgmtPort, theme." and exit 2.

**`oasis settings set <key> <value>`**

- PATCH `/api/v1/settings` with `{ "<key>": <value> }`.
- Validate key client-side before sending. For `mgmtPort`, validate it is a valid port number (1–65535).
- Print "Setting '<key>' updated."

#### `internal/cli/db.go` — `db backup`

Sub-command `oasis db backup [--output <path>]`.

- Default output path: `./oasis-backup-<RFC3339-timestamp>.db`.
- Shell out: `docker cp <containerName>:<OASIS_DB_PATH> <output>`.
- `OASIS_DB_PATH` defaults to `/data/db/oasis.db`; can be overridden with `--db-path` for advanced users.
- Print "Database backed up to <path>." on success.

### Output formatting utilities (`internal/cli/table`)

- `PrintTable(headers []string, rows [][]string, w io.Writer)` — renders a fixed-width table with a header row and separator line; uses `text/tabwriter`.
- `PrintKV(pairs []KVPair, w io.Writer)` — renders a vertical key/value list (two-column tabwriter).
- `Spinner(label string, f func() error) error` — shows an animated spinner on stderr while `f` runs; suppressed when stderr is not a TTY (detect with `golang.org/x/term` or `github.com/mattn/go-isatty`).
- These utilities handle the `--quiet` flag centrally so individual commands don't need to.

### Global flag wiring (`internal/cli/root.go`)

- Add a `newClient()` helper that loads config and returns a `*client.Client` with the configured endpoint and the binary version string. All commands call this helper rather than constructing a client directly.
- The `--config` flag overrides the default path passed to `config.Load`.
- Remove `newStubCmd` once all commands are implemented; replace stub registrations with real command constructors.

---

## Edge Case Considerations

- **`oasis init` run twice:** If `~/.oasis/config.json` exists and the container is already running, abort early with a friendly message and the current status. If the config exists but the container is not running, offer to run `oasis start` instead of re-running the wizard.
- **Docker not installed:** Print a one-sentence install hint ("Install Docker at https://docs.docker.com/get-docker/") and exit 1. Do not print a Go stack trace.
- **Container name conflict on `oasis init`:** If a stopped container with the same name already exists from a previous failed init, `docker run` will fail. Detect this with `docker.ContainerExists` and offer to remove the stale container before proceeding.
- **Management API unreachable:** When the container is running but the API returns a connection error, `oasis status` should print "Controller is not responding — check logs with `oasis logs`" rather than a raw `dial tcp ... connection refused` error.
- **Tailscale join timeout during `oasis init`:** After 90 seconds without `tailscaleConnected: true`, print "Tailscale connection is taking longer than expected. Your oasis container is running but may not be reachable yet. Check status with `oasis status`." and exit 0 (not 1 — the container is running; Tailscale may just be slow).
- **Invalid auth key supplied to `oasis init`:** The controller (via tsnet) will eventually set `tailscaleConnected: false` and the poller will time out. The CLI cannot distinguish an invalid key from a slow connection at the HTTP level; the timeout message above applies. Future work: surface tsnet error details via `/api/v1/status`.
- **`oasis app add` with an unreachable upstream URL:** The API accepts it (health starts as `"unknown"`). The CLI should not pre-validate reachability — it would block registration of apps that start after oasis. However, if the URL is not a valid `http://` or `https://` URL, print a client-side error: "URL must start with http:// or https://".
- **`oasis app update` with no flags:** Exit 2 with usage error "provide at least one flag to update".
- **`oasis settings set mgmtPort <value>`:** If the new port differs from the current `mgmtEndpoint` in config, update `~/.oasis/config.json` automatically after a successful PATCH so subsequent commands use the new port.
- **`oasis update` when the container is stopped:** Pull succeeds, but the restart step will not work. Detect this with `docker.ContainerRunning` and print "Container is not running. Start it first with `oasis start`, then run `oasis update`." and exit 1.
- **`--json` and `--quiet` together:** `--json` takes precedence; suppress all non-error prose but still emit the JSON object. Document this in `--help`.
- **Non-TTY output:** Suppress spinners and ANSI colour codes automatically when stdout/stderr is not a TTY. Use `golang.org/x/term.IsTerminal(int(os.Stderr.Fd()))` to detect.
- **Version skew warning:** If the controller's `version` in `/api/v1/status` differs from the CLI's embedded version, print a warning to stderr: "Warning: CLI version vX differs from controller version vY. Run `oasis update` to sync." Do not block the command.

---

## Test Considerations

### Unit tests (`*_test.go` alongside each package, no build tags)

**`internal/cli/config`**
- `Load` on a missing file returns defaults, not an error.
- `Load` / `Save` round-trips all fields correctly.
- `Save` is atomic: a partial write does not corrupt the existing file.

**`internal/cli/client`**
- Use `httptest.NewServer` for all tests; no real network.
- `Get` decodes a 200 JSON response into the target struct.
- `Get` returns a typed `APIError` with the correct `Code` and `Message` for 400, 404, 409, 500 responses.
- All requests include `X-Oasis-CLI-Version` and `Content-Type: application/json`.
- Connection refused returns an error with a message that does not contain a raw Go dial string.

**`internal/cli/table`**
- `PrintTable` output contains all expected headers and cell values.
- `PrintKV` output contains all expected keys.
- Spinner is a no-op (no panic, no output) when the writer is not a TTY.

**`internal/cli/app` handler logic**
- `app add` command: client-side slug validation rejects `My App` and accepts `my-app`.
- `app add` command: `--url` without `http://` or `https://` prefix is rejected before any API call.
- `app update` command: exits 2 and prints usage error when no update flags are provided.
- `app list` command: prints the empty-state message when the items array is empty.

**`internal/cli/settings`**
- `settings get` with an unknown key exits 2 and names the valid keys.
- `settings set mgmtPort` with `0` or `99999` exits 2 with a validation error.

**`internal/cli/container`**
- `status` command: when the API returns a connection error and `docker.ContainerRunning` is false, prints the "container stopped" message (inject mock docker and client).

### Integration tests (build tag `//go:build integration`)

- `make test-integration` spins up the controller via Docker Compose.
- Full `oasis app` lifecycle: `app add` → `app list` (appears) → `app disable` → `app list` (shows disabled) → `app enable` → `app show` → `app update --name` → `app remove` → `app show` (404).
- `oasis status` returns a valid status object with at least `version` and `nginxStatus` populated.
- `oasis settings get` / `oasis settings set theme dark` / `oasis settings get theme` round-trip.
- `oasis db backup` produces a non-empty file at the specified output path and the file is a valid SQLite database (check `file --mime` or read the 16-byte magic header `53 51 4c 69 74 65`).

---

## Codebase Integration

- All new packages under `internal/cli/` must have a godoc-style `// Package ...` comment and godoc on all exported types and functions (critical invariant #9).
- The Docker interaction layer (`internal/cli/docker`) shells out to the `docker` CLI binary; add `os/exec` usage tests that inject a fake `exec.Cmd` to avoid requiring Docker in unit test environments.
- The `internal/cli/client.Client` must set `X-Oasis-CLI-Version` on every request to satisfy the management API convention in `aspec/architecture/apis.md`.
- Do not add a Docker SDK (`github.com/docker/docker/client`) dependency — shelling out keeps the binary dependency-light. If the Docker SDK is ever needed, it requires a separate work item and aspec update.
- Add `golang.org/x/term` (or `github.com/mattn/go-isatty`) to `go.mod` for TTY detection; run `go mod tidy` after.
- No CGO: all new code must compile with `CGO_ENABLED=0`. `golang.org/x/term` on Linux uses `syscall`, which is CGO-free.
- `--json` output must always be valid JSON parseable by `jq`. Validate this in integration tests with `jq .` via a subprocess call.
- CLI command exit codes: 0 success, 1 runtime error, 2 usage error. Cobra's `RunE` returns non-nil for runtime errors; use `cmd.Usage()` + return `&UsageError{}` sentinel for usage errors so the root command can map these to exit code 2. 
- Follow the command/flag structure in `aspec/uxui/cli.md` exactly — do not add flags or subcommands not listed there unless the aspec is updated first.
- Controller changes are out of scope. If a bug is found in the controller API during integration testing, document it with a failing test and open a follow-up work item rather than patching the controller in this branch.
- Run `golangci-lint run ./internal/cli/... ./cmd/oasis/...` before marking done; fix all reported issues.
