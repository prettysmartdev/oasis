# Work Item: Task

Title: Initial controller/gateway implementation
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary:
- Implement the full controller binary — management API, SQLite persistence, tsnet node, NGINX config generation, and background health checker — as the first end-to-end working slice of oasis.
- The controller must be production-safe with respect to the project's critical invariants: loopback-only management API, graceful NGINX reload, no auth key leakage, and a static CGO-free binary.
- The webapp and CLI are explicitly out of scope; the deliverable is a runnable container in which the controller manages NGINX routes and joins the tailnet.

---

## User Stories

### User Story 1:
As a: Owner / Admin

I want to:
Register a locally-running app with oasis via the management API so that it appears on the dashboard and is reachable over my tailnet.

So I can:
Access all my vibe-coded apps through a single, secure URL on my tailnet without exposing them to the public internet or managing NGINX by hand.

### User Story 2:
As a: Owner / Admin

I want to:
Perform first-time setup by providing my Tailscale auth key once, and have the controller join my tailnet and persist its state across container restarts.

So I can:
Run `oasis init` once and never have to supply the auth key again, even after restarting or updating the container.

### User Story 3:
As a: Owner / Admin

I want to:
Query `/api/v1/status` and see whether the controller is healthy, the tailnet connection is live, NGINX is running, and how many apps are registered.

So I can:
Quickly diagnose problems without shelling into the container or reading raw logs.

---

## Implementation Details:

### Package: `internal/controller/db`

- Open (or create) the SQLite database at `OASIS_DB_PATH` (default `/data/db/oasis.db`) using `modernc.org/sqlite` via `database/sql`.
- Apply schema migrations inline at startup via a `PRAGMA user_version` gate — no migration framework needed at this scale.
- Schema:

  ```sql
  CREATE TABLE IF NOT EXISTS apps (
    id          TEXT PRIMARY KEY,          -- UUID v4
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,      -- [a-z0-9-]+
    upstream_url TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    tags         TEXT NOT NULL DEFAULT '[]', -- JSON array stored as text
    enabled      INTEGER NOT NULL DEFAULT 1,
    health       TEXT NOT NULL DEFAULT 'unknown',
    created_at   TEXT NOT NULL,            -- RFC3339
    updated_at   TEXT NOT NULL             -- RFC3339
  );

  CREATE TABLE IF NOT EXISTS settings (
    id                  INTEGER PRIMARY KEY CHECK (id = 1), -- single-row sentinel
    tailscale_hostname  TEXT NOT NULL DEFAULT 'oasis',
    mgmt_port           INTEGER NOT NULL DEFAULT 4515,
    theme               TEXT NOT NULL DEFAULT 'system'
  );

  INSERT OR IGNORE INTO settings (id) VALUES (1);
  ```

- Exported methods on `Store`: `CreateApp`, `GetApp(slug)`, `ListApps`, `UpdateApp(slug, patch)`, `DeleteApp(slug)`, `SetAppHealth(slug, health)`, `GetSettings`, `UpdateSettings(patch)`.
- All writes use transactions. `UpdateApp` accepts a partial struct (pointer fields or functional options) so PATCH semantics work without a full replacement.
- `Store` must implement `io.Closer`; defer `db.Close()` in `main`.

### Package: `internal/controller/nginx`

- Add `tailscale.com/tsnet` and `github.com/nicholasgasior/crossplane-go` (or the canonical crossplane-go package — confirm import path and run `go get`) to `go.mod`.
  - **Note:** crossplane-go's canonical import path is `github.com/aluttik/go-crossplane`. Verify with `go get` before coding.
- `Configurator` gains fields: `configPath string` (path to write `nginx.conf`, e.g. `/etc/nginx/nginx.conf`), `nginxPID func() (int, error)` (injectable for testing).
- `Configurator.Apply(apps []db.App, tailscaleIP string)` method:
  1. Builds the full NGINX config using go-crossplane's AST types — no `fmt.Sprintf` or text templates.
  2. Writes the config atomically (write to a `.tmp` file, then `os.Rename`).
  3. Sends `SIGHUP` to the NGINX master process to trigger a graceful reload.
  4. Returns an error if config generation or the file write fails; SIGHUP errors are logged but not fatal (NGINX may not be running in dev/test).
- Config structure:
  - `http` block with `server` block listening on the Tailscale interface IP (port 80/443 TBD by tsnet TLS).
  - One `location /apps/<slug>/` block per enabled app → `proxy_pass http://<upstreamURL>`.
  - One `location /` block serving the static Next.js export from `/srv/webapp`.
  - Disabled apps must not produce a location block.
- Write a helper `FindNginxPID()` that reads `/var/run/nginx.pid`.
- Unit tests must not require a running NGINX process; inject a no-op PID finder.

### Package: `internal/controller/tsnet`

- Add `tailscale.com` to `go.mod` (`go get tailscale.com@latest && go mod tidy`).
- `Node` gains fields: `srv *tsnet.Server`, `hostname string`, `stateDir string`.
- `New(hostname, stateDir string) *Node`.
- `Node.Start(ctx context.Context) (net.Listener, error)`:
  - Creates and starts a `tsnet.Server` with `Hostname` and `Dir` set.
  - Returns a `net.Listener` on the tsnet interface for the controller to serve the webapp API.
  - Does **not** log `TS_AUTHKEY`; the tsnet server reads it from the environment directly (`TS_AUTHKEY` env var is how tsnet bootstraps on first run).
- `Node.TailscaleIP() (string, error)` — returns the node's IPv4 tailnet address for use in NGINX config.
- `Node.Close() error` — graceful shutdown.
- `Node.HTTPClient() *http.Client` — returns an HTTP client dialing over the tsnet interface (for future use by the webapp API).
- Tests use a stub interface / build tag (`//go:build integration`) so unit tests never establish real Tailscale connections.

### Package: `internal/controller/api`

Implement all endpoints from `aspec/architecture/apis.md`. `Handler` receives `*db.Store`, `*nginx.Configurator`, and `*tsnet.Node` as dependencies (constructor injection).

**Routes to implement:**

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/api/v1/status` | Returns `Status` object; derive `nginxStatus` from PID file existence |
| `GET` | `/api/v1/apps` | Returns `{ "items": [...], "total": N }` |
| `POST` | `/api/v1/apps` | Validate slug format, check for conflicts (409), persist, trigger NGINX reload |
| `GET` | `/api/v1/apps/:slug` | 404 if not found |
| `PATCH` | `/api/v1/apps/:slug` | Partial update; trigger NGINX reload if `enabled` or `upstreamURL` changed |
| `DELETE` | `/api/v1/apps/:slug` | Trigger NGINX reload |
| `POST` | `/api/v1/apps/:slug/enable` | Set `enabled=true`, trigger reload |
| `POST` | `/api/v1/apps/:slug/disable` | Set `enabled=false`, trigger reload |
| `GET` | `/api/v1/settings` | Never return `tailscaleAuthKey` in response |
| `PATCH` | `/api/v1/settings` | Partial update |
| `POST` | `/api/v1/setup` | Accept `{ "tailscaleAuthKey": "...", "hostname": "..." }`; set `TS_AUTHKEY` env var and start tsnet node; one-time only (return 409 if already configured) |

- Route matching uses Go 1.22 `net/http` pattern syntax (`GET /api/v1/apps/{slug}`).
- Extract `{slug}` via `r.PathValue("slug")`.
- Shared helpers: `writeJSON(w, status, v)`, `writeError(w, status, humanMsg, code)`, `readJSON(r, &v)`.
- `Handler.RegisterRoutes(mux *http.ServeMux)` registers all routes on the provided mux.
- The same `Handler` is used for both the management API mux and the tsnet-facing mux; write-endpoint registration is gated by a `readOnly bool` constructor parameter so the tsnet mux only exposes `GET` routes.

### Package: `internal/controller/health`

- New package `internal/controller/health`.
- `Checker` runs a ticker (default interval: 30 s, configurable) that iterates all enabled apps from the store, makes a `HEAD` (fallback `GET`) request to each `upstreamURL`, and calls `store.SetAppHealth(slug, "healthy"|"unreachable")`.
- Uses `net/http` with a 5-second timeout per request.
- Stops cleanly when its context is cancelled.
- Does not trigger NGINX reloads (health is metadata only, not a routing change).

### `cmd/controller/main.go` — wiring

1. Read all env vars (`OASIS_MGMT_PORT`, `OASIS_HOSTNAME`, `OASIS_DB_PATH`, `OASIS_TS_STATE_DIR`, `OASIS_LOG_LEVEL`).
2. Open `db.Store`.
3. Create `nginx.Configurator`.
4. Create `tsnet.Node` (do not start yet — start is triggered by `/api/v1/setup` or on restart if tsnet state already exists).
5. Create `api.Handler` and call `RegisterRoutes` on both the management mux and a tsnet mux.
6. Start management API server (loopback only).
7. If tsnet state directory is non-empty (node was previously set up), call `node.Start` immediately, then apply the current NGINX config, then start the webapp API server on the tsnet listener.
8. Start `health.Checker`.
9. Block on `ctx.Done()` (SIGINT/SIGTERM); graceful shutdown in reverse order.

---

## Edge Case Considerations:

- **Slug collision on POST /api/v1/apps:** Return `409 Conflict` with `code: "SLUG_CONFLICT"`. Do not auto-generate a variant slug.
- **Invalid slug format:** Return `400 Bad Request` with `code: "INVALID_SLUG"` if the slug does not match `[a-z0-9-]+`.
- **NGINX not running at reload time:** `Configurator.Apply` must not return a fatal error if SIGHUP fails (e.g. during development without NGINX). Log a warning and continue. The config file is still written so NGINX will pick it up on its next start.
- **tsnet not yet configured on startup (no state dir content):** The controller starts without joining the tailnet. The webapp API server is not started. `GET /api/v1/status` reflects `tailscaleConnected: false`. A `POST /api/v1/setup` call bootstraps the node at runtime.
- **`POST /api/v1/setup` called twice:** Return `409 Conflict` with `code: "ALREADY_CONFIGURED"`. Check for existing tsnet state before attempting re-join.
- **`TS_AUTHKEY` must never be logged:** Accept it in the request body, pass it to tsnet, then discard. Never write it to the database, logs, or response bodies. Even on error, log only `"setup failed"` — not the key value.
- **Upstream URL unreachable at registration time:** Allowed — apps may be registered before the upstream is running. Health status starts as `"unknown"` and is resolved by the next health check cycle.
- **`upstreamURL` validation:** Verify it is a valid HTTP/HTTPS URL with a host component at creation time (return `400`). Do not verify reachability.
- **Database path directory missing:** `os.MkdirAll` the parent directory before opening SQLite; log the error and exit if it fails. This handles fresh volume mounts.
- **NGINX config path not writable:** `Configurator.Apply` returns an error; the controller logs it and continues — existing NGINX config (from the previous run) remains active.
- **Concurrent API requests and NGINX reloads:** Serialize NGINX config writes and SIGHUP behind a mutex in `Configurator` to prevent partial writes.
- **Container restart with existing state:** On startup the controller reads all apps from SQLite and calls `Configurator.Apply` once to restore routes, before accepting management API requests. This ensures NGINX is always consistent with the database on boot.

---

## Test Considerations:

### Unit tests (`*_test.go` alongside each package, no build tags)

- **`internal/controller/db`:**
  - Use an in-memory SQLite path (`:memory:`) for all tests.
  - Test `CreateApp` → `GetApp` round-trip preserves all fields.
  - Test slug uniqueness constraint returns the correct error type.
  - Test `UpdateApp` with partial fields leaves untouched fields unchanged.
  - Test `DeleteApp` returns `sql.ErrNoRows` (or a typed sentinel) for a missing slug.
  - Test `GetSettings` returns defaults on a fresh database.

- **`internal/controller/nginx`:**
  - Inject a no-op PID finder to avoid requiring a running NGINX.
  - Test that `Apply` with zero apps produces a config with no `location /apps/` blocks.
  - Test that a disabled app is not present in the generated config.
  - Test that the generated config includes the correct `proxy_pass` for each enabled app.
  - Test atomic write: the config is only overwritten on success (use a temp dir).

- **`internal/controller/api`:**
  - Use `net/http/httptest.NewRecorder()` for all handler tests; no real listener needed.
  - Table-driven tests for each endpoint covering: happy path, 404 not found, 409 conflict, 400 bad request, 405 method not allowed.
  - Test that `POST /api/v1/setup` on a pre-configured node returns 409.
  - Test that `GET /api/v1/settings` response body never contains a `tailscaleAuthKey` field.
  - Test that write endpoints return 405 on the read-only (tsnet) handler.

- **`internal/controller/health`:**
  - Use `httptest.NewServer` to simulate healthy and unreachable upstreams.
  - Verify `SetAppHealth` is called with `"healthy"` for a 200 response and `"unreachable"` for a connection refused.
  - Verify the checker stops promptly when context is cancelled.

- **`cmd/controller/main_test.go`:**
  - Test `buildMgmtAddr` never returns a string starting with `0.0.0.0` (existing invariant test).

### Integration tests (build tag `//go:build integration`)

- `make test-integration` via Docker Compose spins up the full container.
- Verify `GET /api/v1/status` returns 200 with `nginxStatus: "running"` after container start.
- Verify a full app lifecycle: `POST /api/v1/apps` → `GET /api/v1/apps/:slug` → `POST /api/v1/apps/:slug/disable` → NGINX config no longer contains the slug → `DELETE /api/v1/apps/:slug` → `GET` returns 404.
- Verify that the database survives a container restart (stop, start, `GET /api/v1/apps` returns the same apps).

---

## Codebase Integration:

- Follow established conventions, best practices, testing, and architecture patterns from the project's aspec.
- Management API changes must follow the REST conventions in `aspec/architecture/apis.md`.
- All Go packages must have godoc-style package comments and exported godoc on all types and functions (critical invariant #9).
- `CGO_ENABLED=0` must be maintained; use `modernc.org/sqlite`, never `mattn/go-sqlite3`. Verify with `go build -v` that no CGO symbols are referenced.
- Never bind the management API to anything other than `127.0.0.1`; the unit test in `cmd/controller/main_test.go` enforces this and must not be weakened.
- NGINX reload must always use `SIGHUP`, never a process restart. The mutex in `Configurator` prevents concurrent reloads.
- `TS_AUTHKEY` must never appear in logs, responses, or the database — apply `grep -r TS_AUTHKEY` in CI to catch accidental leaks.
- Add `tailscale.com` and `go-crossplane` to `go.mod` via `go get` before implementing `tsnet` and `nginx` packages; run `go mod tidy` after.
- Run `golangci-lint run` before marking the work item done; fix all reported issues.
- Version is embedded at build time via `-ldflags`; `main.version` must be the only source of truth for the version string returned in `/api/v1/status`.
- See `aspec/architecture/security.md` for the full secrets management policy and container security constraints.
- See `aspec/devops/infrastructure.md` for volume mount paths and the non-root uid 1000 requirement.
