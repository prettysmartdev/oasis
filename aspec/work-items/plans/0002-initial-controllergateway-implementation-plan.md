# Plan: 0002 — Initial Controller/Gateway Implementation

Work Item: `aspec/work-items/0002-initial-controllergateway-implementation.md`
Status: Implemented

---

## Overview

Implement the full controller binary as the first end-to-end working slice of oasis. This work item takes the stub packages from work item 0001 and replaces them with production-ready implementations covering SQLite persistence, programmatic NGINX config generation, Tailscale tsnet integration, the complete management API, and a background health check loop.

The deliverable is a runnable controller binary that can:
- Accept app registrations via the management API
- Generate and hot-reload NGINX configuration when the app registry changes
- Join the user's tailnet via tsnet on first setup and automatically reconnect on restart
- Report the health of each registered upstream on a 30-second cycle

The webapp (Next.js) and CLI are explicitly out of scope for this work item.

---

## Open Questions / Decisions Made

1. **`TsnetNode` interface in `internal/controller/api`** — The work item spec called for `*tsnet.Node` to be passed directly to the `Handler`. During implementation an interface (`TsnetNode`) was extracted in `internal/controller/api/handler.go` exposing only `IsStarted()`, `TailscaleIP()`, and `Start(ctx)`. This decouples the handler from the concrete tsnet package, enabling mock injection in unit tests without real Tailscale connections.

2. **`readOnly bool` constructor parameter on `Handler`** — The spec described a tsnet-facing mux that exposes only read endpoints. Rather than a separate handler type, a single `Handler` was used with a `readOnly bool` parameter that gates write-endpoint registration in `RegisterRoutes`. `main.go` creates two `Handler` instances sharing the same `*db.Store`, `*nginx.Configurator`, and `*tsnet.Node`: one for the management mux (`readOnly: false`) and one for the tsnet mux (`readOnly: true`).

3. **Empty `configPath` as test mode signal in `nginx.Configurator`** — Passing an empty string for `configPath` to `NewWithConfig` causes `Apply` to skip the file write and SIGHUP entirely. This is the designated test-mode signal; no separate build tag or interface is needed for nginx tests.

4. **SIGHUP errors are non-fatal** — If NGINX is not running (e.g. during development or in unit tests), the SIGHUP will fail. The controller logs a warning and continues normally; the config file is still written to disk so NGINX will pick it up on its next start.

5. **`isTsnetConfigured` heuristic** — On restart the controller must know whether to start tsnet automatically. The chosen signal is the presence of any `.json` file or a file named `tailscaled.state` in `OASIS_TS_STATE_DIR`. This matches the files tsnet writes to its state directory.

6. **Graceful shutdown order** — Shutdown closes in reverse dependency order: tsnet HTTP server → management HTTP server → tsnet node (`node.Close()`). The health checker stops automatically when its context is cancelled (which happens before the servers are closed).

7. **Health check HEAD-then-GET fallback** — Some app upstreams do not implement HEAD. The checker issues HEAD first; if that fails for any reason (connection error or non-2xx/3xx), it falls back to GET. A 400–599 response from either method is treated as `"unreachable"`.

8. **Timestamp precision** — Timestamps are stored and returned in RFC3339 format at second precision (truncated via `time.Truncate(time.Second)` in the test helper, and stored via `time.RFC3339` formatting, which drops sub-second components).

9. **Tags stored as JSON text** — SQLite has no native array type. Tags are marshalled to a JSON string (`"[\"a\",\"b\"]"`) on write and unmarshalled on read. A malformed JSON value in the database falls back to an empty slice rather than returning an error.

10. **`upstreamURL` trailing slash normalisation in NGINX config** — When building the `proxy_pass` directive, a trailing slash is appended to the upstream URL if absent. This ensures NGINX strips the `/apps/<slug>/` prefix from forwarded requests correctly.

11. **crossplane-go import path** — The canonical import path is `github.com/aluttik/go-crossplane`, not the `nicholasgasior` variant referenced in the work item. This was confirmed via `go get` before implementation.

---

## Implementation Steps

### Phase 1: Package `internal/controller/db`
**Owner:** go-backend
**Files:** `internal/controller/db/store.go`, `internal/controller/db/store_test.go`

#### Step 1.1 — Schema and migration
Define the schema exactly as specified in the work item. Apply it via a `PRAGMA user_version` gate: read the current version; if it is less than 1, run the DDL and set `PRAGMA user_version = 1`. Subsequent opens skip the migration.

Key schema decisions:
- `apps.tags` column is `TEXT NOT NULL DEFAULT '[]'` — JSON array stored as text.
- `apps.enabled` column is `INTEGER` — SQLite has no boolean type; `1 = true`, `0 = false`.
- `apps.health` defaults to `'unknown'`; only the health checker writes `'healthy'` or `'unreachable'`.
- `settings` uses a single-row sentinel (`id INTEGER PRIMARY KEY CHECK (id = 1)`) with `INSERT OR IGNORE` to seed defaults.

#### Step 1.2 — `Store` type and constructor
```
func New(path string) (*Store, error)
```
- Pass `""` or `":memory:"` for an in-memory database (tests).
- Call `db.SetMaxOpenConns(1)` — SQLite performs best with a single writer connection; this prevents WAL contention.
- Run `migrate` before returning; close and return the error if migration fails.

#### Step 1.3 — `App` and `AppPatch` types
`App` carries all columns. `AppPatch` uses pointer fields for every mutable field (nil = no change). This gives PATCH semantics: only the fields the caller supplies are updated; all others are read from the database first inside a transaction and written back unchanged.

#### Step 1.4 — CRUD methods
All methods accept a `context.Context` as the first parameter.
- `CreateApp` — marshals tags to JSON, converts `Enabled` to int.
- `GetApp(slug)` — returns `ErrNotFound` if `sql.ErrNoRows`.
- `ListApps` — ordered by `created_at ASC`.
- `UpdateApp(slug, patch)` — begin transaction, read current row, apply non-nil patch fields, write back, commit.
- `DeleteApp(slug)` — check `RowsAffected`; return `ErrNotFound` if zero.
- `SetAppHealth(slug, health)` — bare `UPDATE`; also updates `updated_at`.
- `GetSettings` / `UpdateSettings(patch)` — same read-modify-write pattern as `UpdateApp`.

#### Step 1.5 — Internal helpers
- `scanApp(scanner)` — accepts both `*sql.Row` and `*sql.Rows` via a local `scanner` interface. Handles `json.Unmarshal` for tags (falling back to `[]string{}`), int-to-bool for `enabled`, and RFC3339 parse for timestamps.
- `boolToInt(bool) int` — converts Go bool to SQLite integer.
- `isUniqueConstraintError(err)` — string-match on `"UNIQUE constraint failed"` for SQLite error detection without importing the driver's concrete error type (placed in `api/handler.go` since that is its only consumer).

---

### Phase 2: Package `internal/controller/nginx`
**Owner:** go-backend
**Files:** `internal/controller/nginx/config.go`, `internal/controller/nginx/config_test.go`

#### Step 2.1 — `Configurator` type and constructors
```
func New() *Configurator
func NewWithConfig(configPath string, nginxPID func() (int, error)) *Configurator
```
`New()` is a convenience wrapper that passes `"/etc/nginx/nginx.conf"` and `FindNginxPID`.

#### Step 2.2 — `Apply(ctx, apps, tailscaleIP)`
1. Call `buildConfig(apps, tailscaleIP)` to generate the config string.
2. If `c.configPath == ""`, return nil immediately (test mode).
3. Write to `configPath + ".tmp"` via `os.WriteFile`.
4. `os.Rename` the temp file to `configPath` (atomic on the same filesystem).
5. Call `c.nginxPID()` to get the NGINX master PID. If it fails, log a warning and return nil.
6. `os.FindProcess(pid)` then `proc.Signal(syscall.SIGHUP)`. If SIGHUP fails, log a warning and return nil (not an error).

#### Step 2.3 — `buildConfig(apps, tailscaleIP)`
Uses the `github.com/aluttik/go-crossplane` AST types exclusively — no `fmt.Sprintf` or text templates:
- Build one `location /apps/<slug>/` directive per enabled app with a `proxy_pass` child. Disabled apps produce no directive.
- Append a catch-all `location /` directive serving `/srv/webapp` via `root` + `try_files`.
- Wrap the location list in a `server` block with `listen <tailscaleIP>:80`.
- Wrap the server block in an `http` block and an `events` block.
- Serialise to a string via `crossplane.Build(&buf, cfg, &crossplane.BuildOptions{Indent: 4})`.

#### Step 2.4 — `FindNginxPID()`
Reads `/var/run/nginx.pid`, trims whitespace, and parses as int.

---

### Phase 3: Package `internal/controller/tsnet`
**Owner:** go-backend
**Files:** `internal/controller/tsnet/node.go`, `internal/controller/tsnet/node_test.go`

#### Step 3.1 — `Node` type and constructors
```
func New() *Node
func NewNode(hostname, stateDir string) *Node
```
`New()` returns a zero-value node for use when only the interface is needed. `NewNode` is used by `main.go`.

All mutable state (`srv`, `started`) is protected by `sync.Mutex`.

#### Step 3.2 — `Start(ctx) (net.Listener, error)`
- Acquire mutex; return an error if already started.
- Create `tsnet.Server{Hostname: hostname, Dir: stateDir}` and call `srv.Start()`.
- Call `srv.Up(ctx)` to wait for the node to be ready. On failure, close the server and return the error.
- Call `srv.Listen("tcp", ":80")` to obtain the listener that `main.go` passes to the tsnet HTTP server.
- Set `n.srv` and `n.started = true`.

`TS_AUTHKEY` is read directly from the environment by the tsnet library. This package never reads, logs, or stores it.

#### Step 3.3 — `TailscaleIP() (string, error)`
- Acquire mutex.
- Return an error if not started.
- Call `srv.TailscaleIPs()`, take the IPv4 address, validate `ip4.IsValid()`.

#### Step 3.4 — `Close() error`
- Acquire mutex; close `n.srv` if non-nil; reset `n.started` and `n.srv`.

#### Step 3.5 — `HTTPClient() *http.Client`
- Returns `n.srv.HTTPClient()` if started, or a plain `&http.Client{}` otherwise. (Reserved for future use by the webapp API.)

#### Step 3.6 — `IsStarted() bool`
- Acquire mutex; return `n.started`.

---

### Phase 4: Package `internal/controller/health`
**Owner:** go-backend
**Files:** `internal/controller/health/checker.go`, `internal/controller/health/checker_test.go`

#### Step 4.1 — `Checker` type and constructor
```
func New(store *db.Store, interval time.Duration) *Checker
```
Creates an internal `*http.Client` with a 5-second timeout.

#### Step 4.2 — `Start(ctx context.Context)`
Runs a `time.NewTicker(c.interval)` loop. On each tick, calls `runChecks(ctx)`. Returns when `ctx.Done()` is closed.

#### Step 4.3 — `runChecks(ctx)`
Lists all apps from the store. For each enabled app, calls `checkApp(ctx, upstreamURL)` and writes the result back via `store.SetAppHealth`. Disabled apps are skipped — their health value is not updated.

#### Step 4.4 — `checkApp(ctx, url) string`
1. Build a HEAD request with `http.NewRequestWithContext`.
2. Execute; if successful and `statusCode` is 200–399, return `"healthy"`.
3. On any error, build a GET request and execute. If successful and 200–399, return `"healthy"`.
4. Otherwise return `"unreachable"`.

---

### Phase 5: Package `internal/controller/api`
**Owner:** go-backend
**Files:** `internal/controller/api/handler.go`, `internal/controller/api/handler_test.go`

#### Step 5.1 — `TsnetNode` interface
```go
type TsnetNode interface {
    IsStarted() bool
    TailscaleIP() (string, error)
    Start(ctx context.Context) (net.Listener, error)
}
```
Defined in `handler.go`. `*tsnetpkg.Node` satisfies this interface; `main.go` required no changes to pass the concrete type.

#### Step 5.2 — `Handler` type and constructor
```
func New(store *db.Store, configurator *nginx.Configurator, node TsnetNode, readOnly bool) *Handler
func (h *Handler) SetVersion(v string)
func (h *Handler) RegisterRoutes(mux *http.ServeMux)
```
All dependency fields may be nil (tests pass nil for unused dependencies). Write endpoints are registered only when `readOnly == false`.

#### Step 5.3 — Route registration (Go 1.22 pattern syntax)
```
GET  /api/v1/status
GET  /api/v1/apps
GET  /api/v1/apps/{slug}
GET  /api/v1/settings
--- (readOnly gates below) ---
POST   /api/v1/apps
PATCH  /api/v1/apps/{slug}
DELETE /api/v1/apps/{slug}
POST   /api/v1/apps/{slug}/enable
POST   /api/v1/apps/{slug}/disable
PATCH  /api/v1/settings
POST   /api/v1/setup
```
Path parameters extracted via `r.PathValue("slug")`.

#### Step 5.4 — Shared helpers
- `writeJSON(w, status, v)` — sets `Content-Type: application/json`, writes status, encodes.
- `writeError(w, status, humanMsg, code)` — writes `{"error":"...","code":"..."}`.
- `readJSON(r, &v)` — decodes and closes the request body.
- `toAppJSON(a db.App) appJSON` — converts the db type to the wire type (camelCase JSON field names, RFC3339 timestamps, nil-safe tags).
- `toSettingsJSON(s db.Settings) settingsJSON` — converts the db settings type to the wire type.

#### Step 5.5 — `triggerNginxReload(ctx)`
Safe to call even when dependencies are nil:
- Return immediately if `h.store == nil || h.nginx == nil || h.node == nil`.
- List all apps from the store.
- Call `h.node.TailscaleIP()`; if it returns an error the node is not yet started — skip reload silently.
- Call `h.nginx.Apply(ctx, apps, ip)`.

NGINX reload is triggered after: `POST /api/v1/apps`, `DELETE /api/v1/apps/{slug}`, `POST /api/v1/apps/{slug}/enable`, `POST /api/v1/apps/{slug}/disable`, and `PATCH /api/v1/apps/{slug}` (only when `enabled` or `upstreamURL` is in the patch).

#### Step 5.6 — Setup endpoint
`POST /api/v1/setup`:
1. Return `503` if `h.node == nil`.
2. Return `409 ALREADY_CONFIGURED` if `h.node.IsStarted()`.
3. Decode request body: `{ "tailscaleAuthKey": "...", "hostname": "..." }`.
4. If `tailscaleAuthKey` is non-empty, call `os.Setenv("TS_AUTHKEY", req.TailscaleAuthKey)`. Never log the value.
5. If `hostname` is non-empty and `h.store != nil`, call `h.store.UpdateSettings` to persist the hostname.
6. Call `h.node.Start(r.Context())`.
7. Return a `statusResponse` identical in shape to `GET /api/v1/status`.

#### Step 5.7 — Status endpoint
`GET /api/v1/status` aggregates: `h.node.IsStarted()`, `h.node.TailscaleIP()`, `h.store.GetSettings()`, `h.store.ListApps()` (for count), and `nginx.FindNginxPID()` (for `nginxStatus`). All field reads are nil-guarded.

---

### Phase 6: `cmd/controller/main.go` — Wiring
**Owner:** go-backend
**File:** `cmd/controller/main.go`

#### Step 6.1 — Startup sequence
1. Read env vars: `OASIS_MGMT_PORT` (default `"04515"`), `OASIS_HOSTNAME`, `OASIS_DB_PATH`, `OASIS_TS_STATE_DIR`, `OASIS_LOG_LEVEL`.
2. Configure `slog` with text handler at the requested level; set as default.
3. Create a cancellable context from `signal.NotifyContext` for `SIGINT` and `SIGTERM`.
4. Open `db.New(dbPath)`.
5. Create `nginx.NewWithConfig("/etc/nginx/nginx.conf", nginx.FindNginxPID)`.
6. Create `tsnetpkg.NewNode(hostname, tsStateDir)`.
7. Create management `api.Handler` (`readOnly: false`) and tsnet `api.Handler` (`readOnly: true`).
8. Listen on `buildMgmtAddr(port)` — always `"127.0.0.1:" + port` — and serve in a goroutine.
9. If `isTsnetConfigured(tsStateDir)`: start the node, apply NGINX config, register tsnet HTTP server on the returned listener, serve in a goroutine.
10. Start `health.New(store, 30*time.Second).Start(ctx)` in a goroutine.
11. Block on `<-ctx.Done()`.

#### Step 6.2 — Graceful shutdown (reverse order)
```
tsnetServer.Close()   // if started
mgmtServer.Close()
node.Close()
```
Store close is deferred at the top of `main`.

#### Step 6.3 — Helper functions
- `buildMgmtAddr(port string) string` — returns `"127.0.0.1:" + port`. The hardcoded `127.0.0.1` host is the loopback invariant enforcement point.
- `envOrDefault(key, def string) string` — returns env value or default.
- `isTsnetConfigured(stateDir string) bool` — reads the state directory; returns true if any entry has a `.json` extension or is named `tailscaled.state`.

#### Step 6.4 — Version embedding
`var version = "dev"` is the sole source of truth; overridden at build time via `-ldflags "-X main.version=..."`.

---

### Phase 7: Unit Tests
**Owner:** go-backend

#### `internal/controller/db/store_test.go`
- `TestCreateGetRoundTrip` — all fields preserved across `CreateApp` → `GetApp`.
- `TestSlugUniqueness` — second insert with same slug returns error containing `"UNIQUE"`.
- `TestUpdateAppPartial` — patching one field leaves all others unchanged.
- `TestDeleteAppNotFound` — returns `ErrNotFound` for a missing slug.
- `TestGetSettingsDefaults` — fresh database returns `hostname: "oasis"`, `mgmtPort: 4515`, `theme: "system"`.

All tests use `New(":memory:")` via a shared `newTestStore(t)` helper.

#### `internal/controller/nginx/config_test.go`
- `TestApplyNoApps` — config generated for zero apps contains no `location /apps/` blocks; fallback `location /` is present.
- `TestApplyDisabledApp` — disabled app produces no `location` block.
- `TestApplyEnabledApp` — enabled app produces the correct `proxy_pass` directive.
- `TestAtomicWrite` — `Apply` with a real temp dir writes to `configPath`, not `configPath.tmp`; the `.tmp` file is absent after success.

All tests inject a no-op PID finder via `NewWithConfig("", func() (int, error) { return 0, errors.New("no nginx") })` or a real temp path.

#### `internal/controller/api/handler_test.go`
- Tests use `httptest.NewRecorder()` and a mock `TsnetNode` implementation.
- Covers: status (no deps, partial deps, full deps), list/get/create/update/delete apps (happy path, 404, 409, 400), enable/disable, get/update settings, setup (happy path, 409 already configured, 503 no node).
- Verifies that `GET /api/v1/settings` response body never contains a `tailscaleAuthKey` field.
- Verifies that write endpoints return `405 Method Not Allowed` on the read-only handler.

#### `internal/controller/health/checker_test.go`
- `TestCheckerHealthy` — upstream server returning `200` results in `"healthy"` stored in the db.
- `TestCheckerUnreachable` — upstream server returning `500` (or connection refused) results in `"unreachable"`.
- `TestCheckerSkipsDisabled` — disabled app health is not updated.
- `TestCheckerStops` — checker exits promptly when context is cancelled.

Tests use `httptest.NewServer` for healthy upstreams and short intervals (e.g. `10ms`) to avoid slow tests.

#### `cmd/controller/main_test.go`
- `TestControllerStartup` — starts a management server on `127.0.0.1:0`, confirms the bound address starts with `127.0.0.1:`, and confirms the server responds (with 404 from an empty mux).
- `TestBuildMgmtAddr` — table-driven test: for each port string, confirms the output is `"127.0.0.1:<port>"` and never starts with `"0.0.0.0"`.

#### `internal/controller/tsnet/node_test.go`
- `TestNodeNew` and `TestNodeNewNode` — constructors return non-nil nodes.
- `TestNodeIsStartedFalse` — fresh node reports `IsStarted() == false`.
- `TestNodeTailscaleIPNotStarted` — returns an error before `Start` is called.
- `TestNodeCloseNotStarted` — `Close()` on an unstarted node returns nil without panicking.

No test establishes a real Tailscale connection. Integration tests requiring a live tailnet are gated with `//go:build integration`.

---

### Phase 8: Integration Tests
**Owner:** devops / go-backend
**Build tag:** `//go:build integration`

These run via `make test-integration` (Docker Compose) and require a real `TS_AUTHKEY`. They are not run in CI on standard push/PR.

- Verify `GET /api/v1/status` returns `200` with `nginxStatus: "running"` after container start.
- Verify a full app lifecycle: `POST /api/v1/apps` → `GET /api/v1/apps/:slug` → `POST /api/v1/apps/:slug/disable` → NGINX config does not contain the slug → `DELETE /api/v1/apps/:slug` → `GET` returns `404`.
- Verify that SQLite data survives a container restart.

---

## File Creation Checklist

```
internal/
  controller/
    db/
      store.go                   (full implementation — replaces 0001 stub)
      store_test.go
    nginx/
      config.go                  (full implementation — replaces 0001 stub)
      config_test.go
    tsnet/
      node.go                    (full implementation — replaces 0001 stub)
      node_test.go
    health/
      checker.go                 (new package)
      checker_test.go
    api/
      handler.go                 (full implementation — replaces 0001 stub)
      handler_test.go
cmd/
  controller/
    main.go                      (full implementation — replaces 0001 stub)
    main_test.go                 (updated with loopback invariant tests)
```

No new files are added to `cmd/oasis/`, `internal/cli/`, or `webapp/` — those are out of scope.

---

## Edge Case Handling Plan

| Edge Case | Handling |
|---|---|
| Slug collision on `POST /api/v1/apps` | Detect `"UNIQUE constraint failed"` in the SQLite error string; return `409 SLUG_CONFLICT`. Do not auto-generate a variant. |
| Invalid slug format | `slugRe = regexp.MustCompile("^[a-z0-9-]+$")` validated before insert; return `400 INVALID_SLUG`. |
| NGINX not running at reload time | `proc.Signal(SIGHUP)` error is logged as a warning; `Apply` returns nil. Config file is written so NGINX will pick it up on next start. |
| tsnet not yet configured on fresh start | `isTsnetConfigured` returns false; tsnet HTTP server is not started. `GET /api/v1/status` returns `tailscaleConnected: false`. `POST /api/v1/setup` bootstraps the node at runtime. |
| `POST /api/v1/setup` called twice | `h.node.IsStarted()` returns true; handler returns `409 ALREADY_CONFIGURED`. |
| `TS_AUTHKEY` leakage | `os.Setenv` is called without logging the value. The variable is accepted in the request body and passed to the tsnet library via env. It is never written to the database, logs, or response bodies. |
| Upstream unreachable at registration | Allowed. Health starts as `"unknown"` and is resolved by the next health check cycle. |
| `upstreamURL` missing at registration | `400 INVALID_UPSTREAM_URL` if the field is empty. URL format validation is not performed beyond non-empty check. |
| `name` missing at registration | `400 INVALID_NAME` if the field is empty. |
| Tags field null in JSON request | Treated as `[]string{}` (not a patch field nil); creates app with empty tags slice. |
| Tags `nil` after db round-trip | `scanApp` falls back to `[]string{}` on unmarshal failure. `toAppJSON` normalises nil to `[]string{}` to guarantee JSON always produces `"tags":[]`. |
| SQLite database directory missing | Not handled inside the `db` package; `main.go` relies on the Docker volume being mounted at the correct path. Error on `sql.Open` causes `os.Exit(1)` with a clear log message. |
| NGINX config path not writable | `os.WriteFile` returns an error; `Apply` returns it; controller logs a warning and continues. |
| Concurrent NGINX reloads | Not currently serialised with a mutex (not added to the implementation). If concurrent requests trigger back-to-back reloads, the `os.Rename` atomic write prevents partial reads but two SIGHUPs may be sent. This is safe — NGINX handles multiple SIGHUPs gracefully. A mutex may be added in a future work item if needed. |
| Container restart with existing state | `isTsnetConfigured` is true; node starts, `Configurator.Apply` is called once with all current apps before the tsnet HTTP server accepts requests. |
| tsnet node fails to start on restart | Controller logs a warning: "failed to start tsnet node (will retry via /api/v1/setup)". Management API continues to serve. User can fix the issue and call `POST /api/v1/setup`. |
| Health checker context cancelled | `ticker.Stop()` is deferred; `select` on `ctx.Done()` causes `Start` to return cleanly. |

---

## Acceptance Criteria

- [x] `CGO_ENABLED=0 go build ./cmd/controller` succeeds
- [x] `file ./bin/controller` reports statically linked
- [x] `go test -race ./...` passes with all packages
- [x] `TestBuildMgmtAddr` confirms management API address always starts with `127.0.0.1:`
- [x] `TestControllerStartup` confirms the management server binds to the loopback interface
- [x] `TestCreateGetRoundTrip` passes — all App fields survive a SQLite round-trip
- [x] `TestSlugUniqueness` passes — duplicate slug returns an error containing `"UNIQUE"`
- [x] `TestUpdateAppPartial` passes — patching one field leaves others unchanged
- [x] `TestDeleteAppNotFound` passes — missing slug returns `ErrNotFound`
- [x] `TestGetSettingsDefaults` passes — fresh database returns expected defaults
- [x] NGINX config for zero apps contains no `location /apps/` blocks and does contain `location /`
- [x] Disabled apps do not appear in the generated NGINX config
- [x] `POST /api/v1/setup` on an already-started node returns `409 ALREADY_CONFIGURED`
- [x] `GET /api/v1/settings` response body never contains a `tailscaleAuthKey` field
- [x] Write endpoints return `405 Method Not Allowed` on the read-only (tsnet) handler
- [x] Health checker marks a 200-response upstream as `"healthy"` and a refused connection as `"unreachable"`
- [x] Health checker exits promptly when its context is cancelled
- [x] `make lint` (`golangci-lint run ./...`) exits 0
- [x] All exported types and functions have godoc comments
