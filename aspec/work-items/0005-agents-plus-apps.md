# Work Item: Feature

Title: Agents plus apps — YAML-file registration and agent scaffold
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary

Extend oasis in two directions: (1) allow apps to be registered from a YAML definition file (in addition to the existing CLI flags), and (2) introduce a new first-class `agent` resource with its own page in the webapp. Agents support three trigger types (icon tap, schedule, webhook), a configurable output format (markdown, HTML, plaintext), and a run-output window that renders results inline. This work item scaffolds the full agent flow end-to-end with stub (dummy) output — no real LLM integration yet — so that the UX, API surface, and data model can be validated and iterated on independently of any AI backend.

---

## User Stories

### User Story 1: YAML-file app registration
As a: Owner / Admin

I want to:
run `oasis app new` to get a filled-out YAML template, edit it in my editor, then run `oasis app add -f ./my-app.yaml` to register the app — and still be able to use the original flag-based `oasis app add` when I prefer

So I can:
version-control my app definitions alongside my projects, share them with teammates, and register apps without remembering every flag name

### User Story 2: Agent registration and management
As a: Owner / Admin

I want to:
register an agent with `oasis agent add` (flags or `-f yaml`), specifying a prompt, trigger type (tap, schedule, or webhook), and output format (markdown, html, or plaintext), and see it appear on the Agents page in the dashboard

So I can:
organize my AI agents alongside my apps on the same homescreen, triggering them in the way that fits each agent's purpose

### User Story 3: Agent run window
As a: Tailnet Visitor

I want to:
tap an agent icon on the Agents page to open an agent window that shows either the live run (for tap-triggered agents) or the most recent run output (for scheduled or webhook-triggered agents), rendered correctly for the agent's configured output format

So I can:
see agent results without leaving the dashboard, and understand the output without raw text getting in the way

---

## Implementation Details

### 1. New `agent` resource — data model

Add a new `Agent` resource, persisted in SQLite alongside apps. Do **not** reuse the `App` table; agents have distinct fields and behavior.

**SQLite schema — new `agents` table:**

```sql
CREATE TABLE agents (
    id          TEXT PRIMARY KEY,          -- UUID v4
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,      -- [a-z0-9-]+
    description TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',  -- emoji or URL
    prompt      TEXT NOT NULL,
    trigger     TEXT NOT NULL,             -- "tap" | "schedule" | "webhook"
    schedule    TEXT NOT NULL DEFAULT '',  -- cron expression; non-empty iff trigger="schedule"
    output_fmt  TEXT NOT NULL DEFAULT 'markdown', -- "markdown" | "html" | "plaintext"
    enabled     BOOLEAN NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL,             -- RFC3339
    updated_at  TEXT NOT NULL              -- RFC3339
);

CREATE TABLE agent_runs (
    id          TEXT PRIMARY KEY,          -- UUID v4
    agent_id    TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    trigger_src TEXT NOT NULL,             -- "tap" | "schedule" | "webhook"
    status      TEXT NOT NULL,             -- "running" | "done" | "error"
    output      TEXT NOT NULL DEFAULT '',
    started_at  TEXT NOT NULL,
    finished_at TEXT                       -- NULL while running
);
```

**Go types (`internal/controller/db/agents.go`):**

```go
type Agent struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Slug        string    `json:"slug"`
    Description string    `json:"description"`
    Icon        string    `json:"icon"`
    Prompt      string    `json:"prompt"`
    Trigger     string    `json:"trigger"`    // "tap" | "schedule" | "webhook"
    Schedule    string    `json:"schedule"`   // cron; only when trigger="schedule"
    OutputFmt   string    `json:"outputFmt"`  // "markdown" | "html" | "plaintext"
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}

type AgentRun struct {
    ID         string     `json:"id"`
    AgentID    string     `json:"agentId"`
    TriggerSrc string     `json:"triggerSrc"`
    Status     string     `json:"status"`
    Output     string     `json:"output"`
    StartedAt  time.Time  `json:"startedAt"`
    FinishedAt *time.Time `json:"finishedAt"`
}
```

Store interface additions in `internal/controller/db/store.go`:
- `CreateAgent(ctx, Agent) error`
- `GetAgent(ctx, slug string) (*Agent, error)`
- `ListAgents(ctx) ([]Agent, error)`
- `UpdateAgent(ctx, slug string, fields map[string]any) error`
- `DeleteAgent(ctx, slug string) error`
- `CreateAgentRun(ctx, AgentRun) error`
- `GetLatestAgentRun(ctx, agentID string) (*AgentRun, error)`
- `GetAgentRun(ctx, runID string) (*AgentRun, error)`
- `UpdateAgentRun(ctx, runID string, status, output string, finishedAt time.Time) error`

### 2. Management API — new agent endpoints

All new agent endpoints live under `/api/v1/agents`. Follow the same conventions as `/api/v1/apps` (see `aspec/architecture/apis.md`).

**New endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/agents` | List all agents; `{ "items": [...], "total": N }` |
| POST | `/api/v1/agents` | Register a new agent |
| GET | `/api/v1/agents/:slug` | Get a single agent |
| PATCH | `/api/v1/agents/:slug` | Update agent fields (partial) |
| DELETE | `/api/v1/agents/:slug` | Remove an agent and its runs |
| POST | `/api/v1/agents/:slug/enable` | Enable a disabled agent |
| POST | `/api/v1/agents/:slug/disable` | Disable an agent |
| POST | `/api/v1/agents/:slug/run` | Trigger a tap-run; creates an `AgentRun`, executes stub, returns `{ "runId": "..." }` |
| GET | `/api/v1/agents/:slug/runs/latest` | Get the most recent `AgentRun` for the agent |
| GET | `/api/v1/agents/runs/:runId` | Get a specific `AgentRun` by ID |

**Webhook trigger endpoint (no auth in this work item):**

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/agents/:slug/webhook` | Public webhook trigger; same behavior as tap-run but `triggerSrc = "webhook"` |

Note: The webhook endpoint is served on **both** the management API and the tsnet (webapp) API so it can be called from outside the host. Document that webhook URLs will be `https://<oasis-hostname>.<tailnet>.ts.net/api/v1/agents/<slug>/webhook`.

**Scheduled trigger:** The controller has an internal scheduler goroutine (see section 4 below) that calls the same run logic with `triggerSrc = "schedule"`. There is no external HTTP endpoint for scheduled triggers.

**POST /api/v1/agents request body:**
```json
{
  "name": "Daily Digest",
  "slug": "daily-digest",
  "description": "...",
  "icon": "📰",
  "prompt": "Summarise the news today.",
  "trigger": "schedule",
  "schedule": "0 8 * * *",
  "outputFmt": "markdown"
}
```

**Validation:**
- `slug` must match `[a-z0-9-]+`; unique; 409 on conflict
- `trigger` must be one of `"tap"`, `"schedule"`, `"webhook"`
- `schedule` is required and must be a valid 5-field cron expression when `trigger = "schedule"`; ignored otherwise
- `outputFmt` must be one of `"markdown"`, `"html"`, `"plaintext"`; defaults to `"markdown"` if omitted

### 3. Stub agent executor (`internal/controller/agent/executor.go`)

The executor is responsible for producing dummy output in the correct format. No LLM integration in this work item.

```go
// Package agent provides stub agent execution. Real AI backends are out of scope for this work item.
package agent

func Run(ctx context.Context, a db.Agent) (output string, err error) {
    switch a.OutputFmt {
    case "markdown":
        return fmt.Sprintf("# Agent: %s\n\n> **Prompt:** %s\n\n_Stub output — AI backend not yet connected._\n\n- Item one\n- Item two\n", a.Name, a.Prompt), nil
    case "html":
        return fmt.Sprintf(`<h1>Agent: %s</h1><blockquote><strong>Prompt:</strong> %s</blockquote><p><em>Stub output — AI backend not yet connected.</em></p><ul><li>Item one</li><li>Item two</li></ul>`, a.Name, a.Prompt), nil
    case "plaintext":
        return fmt.Sprintf("Agent: %s\nPrompt: %s\n\nStub output — AI backend not yet connected.\n\n* Item one\n* Item two\n", a.Name, a.Prompt), nil
    }
    return "", fmt.Errorf("unknown output format: %s", a.OutputFmt)
}
```

The controller handler for `POST /api/v1/agents/:slug/run`:
1. Fetches the agent from DB.
2. Creates an `AgentRun` row with `status = "running"`.
3. Spawns a goroutine: calls `agent.Run`, updates the row to `status = "done"` with the output (or `status = "error"` on failure).
4. Returns `202 Accepted` with `{ "runId": "<uuid>" }` immediately (non-blocking).

The tap-run flow in the webapp polls `GET /api/v1/agents/runs/:runId` until `status != "running"` (1s interval, 5m timeout), shows an animated loading screen.

### 4. Scheduler goroutine (`internal/controller/agent/scheduler.go`)

- Starts on controller boot; reads all enabled schedule-triggered agents from DB.
- Uses a simple ticker loop checking every minute against cron expressions (use `github.com/robfig/cron/v3` — already dependency-light, CGO-free).
- When a schedule fires, calls `agent.Run` and persists the result as an `AgentRun` with `triggerSrc = "schedule"`.
- Re-reads the agent list from DB every minute so new agents are picked up without a restart.
- Does not expose an HTTP interface; runs entirely in process.
- On controller shutdown (context cancellation), drains in-flight runs gracefully (allow up to 10 s).

### 5. YAML definition file — apps and agents

#### App YAML template (generated by `oasis app new <name>`)

File written to `./oasis-app-<name>.yaml` in the current working directory:

```yaml
# OaSis app definition — fill in the fields and run: oasis app add -f ./oasis-app.yaml
name: "<name>"          # Display name (required)
slug: "<name-url-sanitized"          # URL-safe identifier, e.g. "my-app" (required, unique)
upstreamUrl: ""           # Upstream URL, e.g. "http://localhost:3000" (required)
description: ""   # Short description shown in the dashboard (optional)
icon: ""          # Emoji or image URL, e.g. "🚀" or "https://..." (required)
tags: []          # List of tags for grouping, e.g. ["work", "tools"]
```

#### Agent YAML template (generated by `oasis agent new <name>`)

File written to `./oasis-agent.yaml` in the current working directory:

```yaml
# OaSis agent definition — fill in the fields and run: oasis agent add -f ./oasis-agent.yaml
name: "<name>"           # Display name (required)
slug: "<name-url-sanitized>"           # URL-safe identifier, e.g. "my-agent" (required, unique)
description: ""    # Short description shown in the dashboard (optional)
icon: ""           # Emoji or image URL, e.g. "🤖" (required)
prompt: ""         # The agent's instruction prompt (required)
trigger: "tap"     # "tap" | "schedule" | "webhook" (required)
schedule: ""       # Cron expression, e.g. "0 8 * * *" — required when trigger is "schedule"
outputFmt: "markdown"  # "markdown" | "html" | "plaintext"
```

#### YAML parsing (`internal/cli/yaml/`)

- `ParseAppFile(path string) (*AppDefinition, error)` — reads and validates a YAML app file; returns typed errors for missing required fields.
- `ParseAgentFile(path string) (*AgentDefinition, error)` — same for agents.
- Use `gopkg.in/yaml.v3` (add to `go.mod`); no CGO.
- Both parsers return the same validation errors as the CLI flag path so callers can share error-formatting code.

### 6. CLI changes (`internal/cli/`)

#### Modified: `oasis app add`

Add `-f` / `--file <path>` flag. When `-f` is set, all other flags are ignored; the command reads and validates the YAML file, then POSTs to the API. When `-f` is absent, existing flag behavior is unchanged.

#### New: `oasis app new <name>`

Writes `./oasis-app-<name>.yaml` template to the current directory (or `--output <path>` override). Errors if the file already exists unless `--force` is passed.

#### New: `oasis agent` subcommand group

Register under the root cobra command alongside `oasis app`:

- **`oasis agent add`** — flags: `--name` (required), `--slug` (required), `--prompt` (required), `--trigger` (required), `--schedule` (required when trigger=schedule), `--output-fmt` (default: markdown), `--description`, `--icon`; also accepts `-f <path>` for YAML input.
- **`oasis agent new <name>`** — writes `./oasis-agent.yaml` template; same `--output` / `--force` flags as `oasis app new`.
- **`oasis agent list [--json]`** — GET `/api/v1/agents`; table columns: `NAME`, `SLUG`, `TRIGGER`, `OUTPUT FMT`, `STATUS`.
- **`oasis agent show <slug> [--json]`** — GET `/api/v1/agents/:slug`; vertical key/value display.
- **`oasis agent remove <slug>`** — DELETE `/api/v1/agents/:slug`.
- **`oasis agent enable <slug>`** / **`oasis agent disable <slug>`** — POST `/api/v1/agents/:slug/enable` and `/disable`.
- **`oasis agent update <slug>`** — PATCH `/api/v1/agents/:slug`; same flag set as `agent add` minus required markers; exits 2 if no flags provided.

#### Update `aspec/uxui/cli.md`

Add all new `oasis app new`, `oasis app add -f`, and `oasis agent *` commands to the CLI spec. (This work item includes updating the spec file alongside the code.)

### 7. Webapp changes (`webapp/`)

#### Agents page — agent icon behavior

When a user taps an agent icon:
- For `trigger = "tap"`: immediately POST `/api/v1/agents/:slug/run` and open the agent window, polling the run until complete.
- For `trigger = "schedule"` or `trigger = "webhook"`: open the agent window showing the most recent run from `GET /api/v1/agents/:slug/runs/latest`; if no run exists yet, show an empty state ("No runs yet — this agent hasn't been triggered yet.", show expected next run time if scheduled).

#### New component: `AgentWindow` (`webapp/components/AgentWindow.tsx`)

A full-screen modal (or slide-up sheet on mobile) with:
- Agent name and icon in the header.
- Trigger badge (Tap / Schedule / Webhook) and last-run timestamp.
- Run output area — rendered differently per `outputFmt`:
  - `markdown`: rendered as HTML using `react-markdown` with `remark-gfm`; styled to match the OaSis dark theme.
  - `html`: rendered via a sandboxed `<iframe srcdoc>` to isolate agent HTML from the app shell.
  - `plaintext`: rendered in a `<pre>` block with `font-family: Geist Mono`; terminal-style dark background.
- While `status = "running"` (tap-triggered only): show an animated spinner/pulse with "Agent running…" message.
- Error state: show `"Agent run failed."` with the error text if `status = "error"`.
- Close button (X) in the top-right corner.

**New packages (add to `package.json`):**
- `react-markdown` + `remark-gfm` — markdown rendering
- No new packages needed for HTML (use `<iframe srcdoc>`) or plaintext (use `<pre>`)

#### Webapp API additions

The Next.js static export fetches from the tsnet API. Add these to the client-side fetch layer (`webapp/lib/api.ts` or equivalent):
- `getAgents()` — GET `/api/v1/agents`
- `triggerAgentRun(slug)` — POST `/api/v1/agents/:slug/run` → returns `{ runId }`
- `getAgentRun(runId)` — GET `/api/v1/agents/runs/:runId`
- `getLatestAgentRun(slug)` — GET `/api/v1/agents/:slug/runs/latest`

---

## Edge Case Considerations

- **YAML file: missing required fields** — `ParseAppFile` / `ParseAgentFile` must return a descriptive error naming every missing required field in one message (e.g. "missing required fields: name, slug, url"). Do not fail on the first missing field.
- **YAML + flags conflict** — When `-f` is passed alongside other flags (e.g. `--name`), print a warning to stderr ("Flags ignored when -f is provided") and proceed with the file. Do not error; the file is authoritative.
- **`oasis app new` / `oasis agent new` — file already exists** — Print "File ./oasis-app.yaml already exists. Use --force to overwrite." and exit 1.
- **Agent slug collision** — Return 409 with `{ "error": "...", "code": "SLUG_CONFLICT" }` as with apps.
- **`trigger = "schedule"` with missing or invalid cron** — Validate the cron expression server-side (not just in the CLI); reject with 400 and `"code": "INVALID_SCHEDULE"`. Valid: standard 5-field cron (`* * * * *`); reject 6-field or non-standard extensions.
- **Tap-triggered agent already running** — If an `AgentRun` with `status = "running"` exists for the agent, the `POST /api/v1/agents/:slug/run` endpoint returns 409 with `"code": "RUN_IN_PROGRESS"` and the `runId` of the in-flight run. The webapp should open the agent window for that existing run instead of starting a new one.
- **Agent run poll timeout in webapp** — If polling exceeds 30 seconds without a terminal status, show "Agent is taking longer than expected — check back later." and stop polling. The run record remains in DB and will appear as the latest run on next open. show a 'check again' button which can be pressed only every 5 seconds.
- **Schedule fires while agent is disabled** — The scheduler must skip disabled agents; check `enabled` flag before executing.
- **Webhook endpoint CSRF** — The webhook endpoint requires no auth in this work item (Tailscale network-layer controls access). Document this clearly. Do not add a CSRF token or secret in this work item; that is future work.
- **Agent deleted while run is in flight** — The goroutine holds a reference to the `AgentRun` ID; even if the agent row is deleted, the run goroutine should complete and attempt to update the run row (which will fail gracefully via foreign key cascade delete). Log the error at debug level; do not panic.
- **HTML output injection** — The `<iframe srcdoc>` sandbox attribute (`sandbox="allow-scripts"`) prevents the agent's HTML from accessing the parent frame's DOM, cookies, or localStorage. Enforce this regardless of output format.
- **`oasis agent list` with no agents** — Print "No agents registered yet. Use `oasis agent add` to register one." (mirror the app empty state message).
- **Schedule agent on controller restart** — The scheduler goroutine loads all schedule agents at startup. It should not re-execute a schedule that already fired in the current window (i.e., if `0 8 * * *` ran at 08:00 and the controller restarts at 08:05, the schedule must not fire again until 08:00 the next day). Implement this by comparing the agent's last `started_at` run timestamp against the current cron window.

---

## Test Considerations

### Unit tests

**`internal/controller/db` (agents)**
- `CreateAgent` persists all fields and is retrievable by slug.
- `ListAgents` returns agents in creation order.
- `DeleteAgent` cascades to `agent_runs`.
- `GetLatestAgentRun` returns the most recently started run for a given agent ID.
- Slug uniqueness: a second `CreateAgent` with the same slug returns a conflict error.

**`internal/controller/api` (agent handlers)**
- Use `httptest.NewServer` for all handler tests.
- `POST /api/v1/agents` with valid body returns 201 and the created agent.
- `POST /api/v1/agents` with duplicate slug returns 409 with `SLUG_CONFLICT`.
- `POST /api/v1/agents` with `trigger = "schedule"` and missing `schedule` field returns 400 with `INVALID_SCHEDULE`.
- `POST /api/v1/agents` with invalid `outputFmt` returns 400.
- `POST /api/v1/agents/:slug/run` on a running agent returns 409 with `RUN_IN_PROGRESS` and the existing `runId`.
- `DELETE /api/v1/agents/:slug` returns 204; subsequent GET returns 404.
- `POST /api/v1/agents/:slug/enable` / `/disable` toggle the `enabled` field.

**`internal/controller/agent`**
- `executor.Run` returns non-empty output for each of the three `outputFmt` values.
- `executor.Run` output for `"markdown"` starts with `#` (heading).
- `executor.Run` output for `"html"` contains `<h1>` and `</h1>`.
- `executor.Run` output for `"plaintext"` contains no HTML tags.
- `executor.Run` with unknown `outputFmt` returns an error.

**`internal/controller/agent/scheduler`**
- Scheduler skips disabled agents.
- Scheduler does not re-fire a schedule in the same cron window after a restart (mock the clock and last-run timestamp).

**`internal/cli/yaml`**
- `ParseAppFile` errors on missing `name`, `slug`, `url` and names all three in one error.
- `ParseAgentFile` errors on missing `name`, `slug`, `prompt`, `trigger`.
- `ParseAgentFile` errors on `trigger = "schedule"` with empty `schedule`.
- `ParseAgentFile` accepts `outputFmt` defaulting to `"markdown"` when omitted.
- Valid files round-trip without error.

**`internal/cli` (app add with -f)**
- `oasis app add -f <yaml>` calls the correct POST endpoint with correctly mapped fields.
- `oasis app add -f` with a missing required field prints the multi-field error without making an API call.
- `oasis app new` writes the template file; errors if file exists without `--force`.

**`internal/cli` (agent commands)**
- `oasis agent add` exits 2 if `--name`, `--slug`, `--prompt`, or `--trigger` is missing.
- `oasis agent add --trigger schedule` exits 2 if `--schedule` is missing.
- `oasis agent update <slug>` with no flags exits 2.
- `oasis agent list` prints empty-state message when items array is empty.

### Integration tests (`//go:build integration`)

- Full agent lifecycle: `agent add` (tap) → `agent list` (appears) → `agent run` via POST → poll until done → check output non-empty → `agent disable` → `agent list` (shows disabled) → `agent remove` → `agent show` (404).
- Full app YAML lifecycle: `oasis app new` → edit slug/name/url in the generated file → `oasis app add -f ./oasis-app.yaml` → `oasis app list` (appears) → `oasis app remove`.
- Webhook trigger: POST to `/api/v1/agents/:slug/webhook` → GET `/api/v1/agents/:slug/runs/latest` shows a completed run.
- Schedule trigger: create a schedule agent with a cron that fires every minute; advance mock clock or wait and verify a run is created (keep this test opt-in with a build tag or env var guard to avoid slow CI).

---

## Codebase Integration

- All new Go packages (`internal/controller/agent/`, `internal/cli/yaml/`) must have godoc-style `// Package ...` comments and godoc on all exported types and functions (critical invariant #9).
- The `agents` table migration must use the same SQLite migration pattern as the `apps` table in `internal/controller/db/`; do not use a third-party migration library — run `CREATE TABLE IF NOT EXISTS` at startup.
- The scheduler goroutine must be started via the controller's main context and terminated cleanly when the context is cancelled; no goroutine leaks.
- `github.com/robfig/cron/v3` — add to `go.mod` and `go.sum`; it is CGO-free and satisfies invariant #2.
- `gopkg.in/yaml.v3` — add to `go.mod`; CGO-free.
- Agent API handlers must be registered on both the management API server and the tsnet API server, with the exception of write/admin endpoints (`POST /agents`, `PATCH`, `DELETE`, `enable`, `disable`) which are management-API-only; the tsnet server exposes read endpoints (`GET /agents`, `GET /agents/:slug`, `GET /agents/:slug/runs/latest`, `GET /agents/runs/:runId`) and the webhook endpoint only.
- New CLI commands must be added to `aspec/uxui/cli.md` as part of this work item (not in a follow-up).
- The `AgentWindow` component must respect `prefers-reduced-motion` for its open/close animation.
- The `<iframe srcdoc>` for HTML output must include `sandbox="allow-scripts"` — do not omit this attribute even for internal/trusted agents.
- `react-markdown` and `remark-gfm` — add to `webapp/package.json`; these are the only new frontend packages permitted for this work item.
- Controller changes do not affect NGINX config generation (agents are not proxied by NGINX — they are run-and-display, not upstream-proxy resources); do not add NGINX route logic for agents.
- Run `golangci-lint run ./internal/controller/... ./internal/cli/... ./cmd/...` before marking done; fix all reported issues.
- Run `npm run lint` and `tsc --noEmit` in `webapp/` before marking done.
