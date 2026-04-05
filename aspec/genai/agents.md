# Agents

Note: Oasis itself does not use LLM-based AI agents internally. It is a platform for hosting, organizing, and accessing AI agents and apps. The genai/agents.md spec is not applicable to the core system.

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
