# Agents

Note: Oasis does not integrate with LLM backends yet. The Agent resource and executor are present as a stub; the prompt field is stored but not sent to any external model. Full LLM integration is a future work item. Oasis is a platform for hosting, organizing, and accessing AI agents and apps.

## Agent Resource

An Agent is a first-class resource (separate from App) that represents a user-defined automated task. Agents are registered via the management API or a YAML definition file and appear on the Agents page of the dashboard.

Fields:
- id — UUID v4, assigned at creation
- name — display name
- slug — URL-safe identifier, [a-z0-9-]+, unique across all agents
- description — optional human-readable description
- icon — optional emoji or URL
- prompt — the instruction text that will be sent to an LLM backend (stored now; used when LLM integration lands)
- trigger — one of: tap, schedule, webhook
- schedule — 5-field cron expression; required only when trigger=schedule
- outputFmt — one of: markdown, html, plaintext; default: markdown
- enabled — bool; disabled agents are skipped by the scheduler and reject webhook calls
- createdAt, updatedAt — RFC3339 timestamps

## Trigger Types

### tap
- Agent is run on demand only.
- Fired via POST /api/v1/agents/:slug/run (management API) or from the dashboard Run button.
- No automatic invocation.

### schedule
- Agent fires automatically on a cron schedule (field: schedule, 5-field cron expression).
- Cron parsing uses github.com/robfig/cron/v3.
- The controller runs a background goroutine (scheduler) that ticks every minute and evaluates all schedule-triggered agents.
- An agent is not re-fired if a run is already in the running state when its cron window opens.
- Disabled agents are always skipped by the scheduler.

### webhook
- Agent is triggered by an HTTP POST to /api/v1/agents/:slug/webhook.
- This endpoint is served on both the management API and the tsnet (webapp) API, so external callers on the tailnet can fire it.
- Returns 202 { "runId": "<uuid>" } immediately; processing is asynchronous.
- If the agent is disabled, returns 409 AGENT_DISABLED.
- If a run is already active, returns 409 RUN_IN_PROGRESS with the active runId.

## Scheduler Behavior

- Implemented as a single background goroutine in the controller; starts at controller startup and stops on graceful shutdown.
- Tick interval: 1 minute.
- On each tick: iterate over all enabled agents with trigger=schedule; check if the current minute matches the cron expression; if yes and no run is currently active for that agent, start a new run.
- Cron windows are evaluated per-minute; an agent will not be double-fired within the same cron window.
- Scheduler skips agents where enabled=false.
- Scheduler does not start a new run if the agent already has a run in status=running.

## Run Lifecycle

A run transitions through these states:
- running — set immediately when the run is created; the executor is called asynchronously
- done — set when the executor completes successfully; output field is populated
- error — set when the executor returns an error; output may contain an error description

Fields:
- id — UUID v4
- agentId — UUID of the parent agent
- triggerSrc — one of: tap, schedule, webhook (matches how this run was started)
- status — running | done | error
- output — result string; format is determined by the agent's outputFmt field
- startedAt — RFC3339 timestamp, set at run creation
- finishedAt — RFC3339 timestamp, set when status transitions to done or error; omitted while running

## Stub Executor

- The current executor does not call any LLM backend.
- It generates a deterministic dummy output in the agent's configured outputFmt.
- Markdown: returns a fenced markdown block acknowledging the prompt.
- HTML: returns a minimal HTML snippet.
- Plaintext: returns a plain-text string.
- The executor always transitions the run to done (never errors in the stub implementation).
- This will be replaced by a real LLM dispatch layer in a future work item.

## Output Rendering (Webapp)

- markdown — rendered with react-markdown + remark-gfm in the AgentWindow component
- html — rendered in a sandboxed iframe using srcdoc (sandbox="allow-scripts"); no external network access
- plaintext — rendered in a `<pre>` element using Geist Mono font

## Background Services (rule-based, not LLM-based)

### App Health Monitor
Name: Health Check Loop
Purpose: Periodically verify that each registered app's upstream URL is reachable and update its health status in the dashboard
Model: N/A (deterministic, rule-based — not an LLM agent)
Provider: N/A
Description:
- A background goroutine in the controller that performs an HTTP GET to each enabled app's upstreamURL on a configurable interval (default: 30 seconds)
- Updates the app's health field in SQLite to "healthy", "unreachable", or "unknown"
- The controller pushes health state changes to NGINX config (e.g. to show a maintenance page for unreachable apps) and to the webapp API response
Guidance:
- Use a configurable timeout per health check (default: 5 seconds); do not let slow upstreams block the check loop
- Health checks run concurrently across all registered apps using a worker pool
- Do not use exponential backoff for health checks — the interval is fixed and user-configurable
- A single failed check does not immediately mark an app as unreachable; require N consecutive failures (default: 2) before changing state
- Log health state transitions at the info level; do not log every successful check (too noisy)
