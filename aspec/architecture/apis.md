# APIs

## Management API (local-only)

Convention: rest
Protocol: http (plain HTTP — no TLS required; traffic never leaves the host loopback interface)
Base path: /api/v1
Bind address: 127.0.0.1:04515 (port configurable via OASIS_MGMT_PORT)

## Webapp API (over tailnet)

Convention: rest
Protocol: https (Tailscale / tsnet provides TLS via Tailscale-issued certificates)
Base path: /api/v1
Bind address: tsnet interface (oasis Tailscale node IP)
Note: Same API surface as the management API for read operations; write/admin endpoints are management-API-only

## Design:

Versioning:
- All endpoints are prefixed with /api/v1
- Breaking changes require a new version prefix (/api/v2); old versions are supported for at least one minor release
- The CLI sends an X-Oasis-CLI-Version header on every request; the controller logs a warning if versions are incompatible

Objects:
- App: { id (uuid), name (string), slug (string, URL-safe), upstreamURL (string), displayName (string), description (string), icon (string, URL or emoji), tags ([]string), enabled (bool), health ("healthy"|"unreachable"|"unknown"), accessType ("direct"|"proxy"), createdAt (RFC3339), updatedAt (RFC3339) }
  - `accessType: "direct"` — tapping the icon opens the upstream URL in a new browser tab (no NGINX proxy route is created)
  - `accessType: "proxy"` — NGINX reverse-proxies `/apps/<slug>/` to the upstream; tapping the icon opens a full-screen iFrame inside the oasis dashboard. `X-Frame-Options` and `Content-Security-Policy` response headers are stripped by NGINX so the browser can embed the upstream. **Known limitation:** upstream apps that hard-code the root path `/` in asset references may break when served under a path prefix (e.g. `/apps/my-app/`).
  - Note: the webapp uses the presence of `"agent"` in `tags` to decide which dashboard page an item appears on — items tagged `"agent"` appear on the Agents page, all others on the Apps page.
- Agent: { id (uuid), name (string), slug (string, URL-safe), description (string), icon (string, URL or emoji), prompt (string), trigger ("tap"|"schedule"|"webhook"), schedule (string, cron expression — present only when trigger="schedule"), outputFmt ("markdown"|"html"|"plaintext"), model (string, optional — Claude model ID, e.g. "claude-opus-4-6"; empty string means use the CLI default), enabled (bool), createdAt (RFC3339), updatedAt (RFC3339) }
- ChatMessage: { id (uuid), role ("user"|"assistant"), content (string), createdAt (RFC3339) }
- AgentRun: { id (uuid), agentId (uuid), triggerSrc ("tap"|"schedule"|"webhook"), status ("running"|"done"|"error"), output (string), startedAt (RFC3339), finishedAt (RFC3339, omitted if still running) }
- Settings: { tailscaleHostname (string), mgmtPort (int), theme ("dark"|"light"|"system") }
  - Note: tailscaleAuthKey is write-only; never returned in GET responses
- Status: { tailscaleConnected (bool), tailscaleIP (string), tailscaleHostname (string), nginxStatus ("running"|"stopped"|"error"), registeredAppCount (int), version (string) }

Authentication:
- Management API: no application-level authentication; host OS access (localhost reachability) is the authorization boundary
- Webapp API: Tailscale handles network-layer device authentication; no tokens or sessions required at the application level
- tailscaleAuthKey is accepted only on POST /api/v1/setup during initial configuration; never stored in plaintext (Tailscale state directory is persisted instead)

Conventions:
- JSON request and response bodies; Content-Type: application/json
- Standard HTTP status codes: 200 (ok), 201 (created), 204 (no content), 400 (bad request), 404 (not found), 409 (conflict), 500 (internal error)
- Error responses: { "error": "human-readable message", "code": "SNAKE_CASE_CODE" }
- List responses: { "items": [...], "total": N }
- PATCH for partial updates (only send fields to change), PUT for full replacement
- All resource IDs are UUIDs (v4)
- Slugs must match [a-z0-9-]+ and are unique; used as stable identifiers in routes and CLI commands

## Endpoint Summary:

### Status
- GET /api/v1/status — controller health, Tailscale connection state, NGINX status, version

### Apps
- GET    /api/v1/apps           — list all registered apps
- POST   /api/v1/apps           — register a new app
- GET    /api/v1/apps/:slug     — get a single app by slug
- PATCH  /api/v1/apps/:slug     — update app fields
- DELETE /api/v1/apps/:slug     — remove an app
- POST   /api/v1/apps/:slug/enable  — enable a disabled app
- POST   /api/v1/apps/:slug/disable — disable an app (hidden from dashboard, route removed from NGINX)

### Agents

Management-API-only endpoints (write):
- POST   /api/v1/agents                       — register a new agent; 409 SLUG_CONFLICT if slug already taken
- PATCH  /api/v1/agents/:slug                 — update agent fields (partial update; only provided fields are changed)
- DELETE /api/v1/agents/:slug                 — remove agent and all associated run history; 404 if not found
- POST   /api/v1/agents/:slug/enable          — enable a disabled agent; 404 if not found
- POST   /api/v1/agents/:slug/disable         — disable an agent; 404 if not found
- POST   /api/v1/agents/:slug/run             — trigger an immediate tap-run; returns 202 { "runId": "<uuid>" }; 409 RUN_IN_PROGRESS if a run is already active (response includes runId of the active run)

Both management and tsnet API endpoints (read + webhook):
- GET    /api/v1/agents                       — list all agents; returns { "items": [...], "total": N }
- GET    /api/v1/agents/:slug                 — get a single agent; 404 if not found
- POST   /api/v1/agents/:slug/webhook         — public webhook trigger; returns 202 { "runId": "<uuid>" }; 409 RUN_IN_PROGRESS if already active; 409 AGENT_DISABLED if agent is disabled
- GET    /api/v1/agents/:slug/runs/latest     — most recent AgentRun for the agent; 404 if none exist
- GET    /api/v1/agents/runs/:runId           — get a specific AgentRun by id; 404 if not found

Validation rules:
- accessType must be one of: direct, proxy; defaults to "proxy" when omitted; error code INVALID_ACCESS_TYPE (400)
- trigger must be one of: tap, schedule, webhook; error code INVALID_TRIGGER (400)
- schedule is required when trigger=schedule; must be a valid 5-field cron expression; error code INVALID_SCHEDULE (400)
- outputFmt must be one of: markdown, html, plaintext; defaults to markdown; error code INVALID_OUTPUT_FMT (400)
- slug must match [a-z0-9-]+; must be unique across all agents; error code SLUG_CONFLICT (409)

YAML definition file (app add -f / agent add -f):

App YAML fields:
```yaml
name:        <string, required>
slug:        <string, required, [a-z0-9-]+>
upstreamUrl: <string, required, HTTP/HTTPS URL>
description: <string, optional>
icon:        <string, optional, emoji or URL>
tags:        <[]string, optional>
accessType:  <string, optional — "direct"|"proxy"; default "proxy">
             # "direct" opens the upstream URL in a new tab (no NGINX route created)
             # "proxy" reverse-proxies /apps/<slug>/ and opens app in an iFrame
```

Agent YAML fields:
```yaml
name:       <string, required>
slug:       <string, required, [a-z0-9-]+>
prompt:     <string, required>
trigger:    <string, required — tap|schedule|webhook>
schedule:   <string, required when trigger=schedule — 5-field cron expression>
outputFmt:  <string, optional — markdown|html|plaintext; default: markdown>
model:      <string, optional — Claude model ID, e.g. "claude-opus-4-6"; omit or leave empty to use the default>
description:<string, optional>
icon:       <string, optional, emoji or URL>
```

### Chat

Both management and tsnet API endpoints (user-facing, not admin-only):
- POST /api/v1/chat/messages    — send a user message; invokes claude synchronously; returns both the user and assistant ChatMessage. Request body: { "message": string }. Response (200): { "userMessage": ChatMessage, "assistantMessage": ChatMessage }. Returns 400 INVALID_MESSAGE if message is empty; 503 EXECUTOR_UNAVAILABLE if the claude binary is not found; 504 CHAT_TIMEOUT if claude does not respond within OASIS_CHAT_TIMEOUT.
- GET  /api/v1/chat/messages    — return the full chat history ordered by creation time; { "items": [...], "total": N }

### Settings (management API only)
- GET   /api/v1/settings        — get current settings (authKey omitted)
- PATCH /api/v1/settings        — update settings

### Setup (management API only, one-time)
- POST /api/v1/setup            — provide initial Tailscale auth key and hostname; triggers tsnet join. Optional field: `claude_oauth_token` (string) — Claude OAuth token stored in memory only (never persisted, never logged, lost on container restart). Passing this field injects CLAUDE_CODE_OAUTH_TOKEN into every claude subprocess invoked by the controller.
