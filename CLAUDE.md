# CLAUDE.md — oasis

> **The `aspec/` folder is the single source of truth for all decisions in this project.**
> Before implementing anything, read the relevant spec files. Never invent conventions not found there.

---

## Project Identity

**Name:** oasis
**Purpose:** A self-hosted homescreen for vibe-coded apps and AI agents — discovers, organizes, and exposes locally-running services exclusively over the user's Tailscale network (tailnet).
**Type:** Single Docker container, self-hosted SaaS

---

## Tech Stack

| Layer | Tech |
|---|---|
| Frontend | TypeScript, Next.js (App Router), Tailwind CSS, shadcn/ui |
| Controller | Go, net/http, tsnet, crossplane-go, modernc.org/sqlite |
| CLI | Go, cobra, net/http |
| Gateway | NGINX (dynamically configured by controller) |
| Process supervisor | s6-overlay |
| Database | SQLite (modernc.org/sqlite — CGO-free) |
| Build | Make, Docker multi-stage |
| CI | GitHub Actions |

---

## Repository Layout

```
cmd/
  controller/main.go      # Controller binary entry point
  oasis/main.go           # CLI binary entry point
internal/
  controller/
    api/                  # Management API handlers
    db/                   # SQLite store
    nginx/                # NGINX config generation (crossplane-go)
    tsnet/                # Tailscale/tsnet integration
  cli/
    root.go               # Cobra root command + global flags
    app.go                # `oasis app` subcommand group
    settings.go           # `oasis settings` subcommand group
webapp/                   # Next.js App Router (static export)
  app/
  components/
.github/workflows/
  ci.yml                  # build + lint + test on push/PR
  release.yml             # multi-arch image + CLI binaries on semver tag
Dockerfile                # multi-stage: node:20-alpine → golang:1.22-alpine → debian:bookworm-slim
docker-compose.dev.yml
Makefile
.env.local.example        # canonical env var list
aspec/                    # ← source of truth; never change code without reading this first
```

---

## Critical Invariants

These must **never** be violated. CI enforces most of them.

1. **Loopback-only management API** — always bind to `127.0.0.1`, never `0.0.0.0`. Enforced in unit tests.
2. **CGO_ENABLED=0** — all Go binaries must be fully static. Use `modernc.org/sqlite`, never `mattn/go-sqlite3`.
3. **Non-root container** — all processes run as uid 1000 inside the container.
4. **TS_AUTHKEY never logged** — not in logs, not in API responses, not in the database.
5. **NGINX reload is graceful** — always use SIGHUP, never restart the process to apply config changes.
6. **Version embedded at build time** — `-ldflags "-X main.version=$(git describe --tags --always)"` in Makefile and CI.
7. **Conventional commits** — `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:` — required for changelog generation.
8. **No dynamic routes or server actions in Next.js** — `output: 'export'` is a hard constraint; static export only.
9. **Go packages must have godoc-style package comments** on all exported types and functions from the start.
10. **`.env.local` must never be committed** — `.gitignore` enforces this; CI must never require `TS_AUTHKEY`.

---

## Architecture at a Glance

```
[oasis CLI] --HTTP--> [127.0.0.1:04515 Management API]
                              |
                         [Controller (Go)]
                         /      |       \
                   [SQLite]  [tsnet]  [crossplane-go]
                                |           |
                           [tailnet]    [NGINX reload]
                                |           |
                          [Tailnet browser] [Registered app upstreams]
```

- **Two HTTP servers in the controller:** management API on `127.0.0.1:04515` (CLI), webapp API on tsnet interface (dashboard).
- **NGINX is the only data plane** — the controller never proxies traffic directly.
- **SQLite is the only database** — persisted via Docker volume `oasis-db`.
- **Tailscale tsnet state** — persisted via Docker volume `oasis-ts-state`.

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `TS_AUTHKEY` | — | Tailscale auth key (first start only) |
| `OASIS_MGMT_PORT` | `04515` | Management API port |
| `OASIS_HOSTNAME` | `oasis` | Tailscale node hostname |
| `OASIS_DB_PATH` | `/data/db/oasis.db` | SQLite database path |
| `OASIS_TS_STATE_DIR` | `/data/ts-state` | tsnet state directory |
| `OASIS_LOG_LEVEL` | `info` | Log verbosity |

---

## API Conventions

Base path: `/api/v1`
Content-Type: `application/json`
Error format: `{ "error": "human-readable", "code": "SNAKE_CASE_CODE" }`
List format: `{ "items": [...], "total": N }`
IDs: UUIDs (v4). Slugs: `[a-z0-9-]+`, unique.
CLI sends `X-Oasis-CLI-Version` header on every request.

**Endpoints:** See `aspec/architecture/apis.md` for the full endpoint list.

---

## CLI Conventions

- Global flags on all commands: `--config`, `--json`, `--quiet`, `--version`
- Stdout: human-readable tables by default; JSON when `--json`
- Stderr: all errors, warnings, progress
- Exit codes: 0 success, 1 error, 2 usage error
- Fully non-interactive except `oasis init`
- Config file: `~/.oasis/config.json`

**Commands:** See `aspec/uxui/cli.md` for the full command list.

---

## Makefile Targets

| Target | Action |
|---|---|
| `make install-tools` | Install `air`, `golangci-lint` via `go install` |
| `make dev` | Run controller (air) + Next.js dev in parallel (`-j2`) |
| `make build` | Build controller + CLI to `./bin/` |
| `make build-cli` | Build CLI only → `./bin/oasis` |
| `make test` | `go test -race ./...` + `npm test --ci` |
| `make lint` | `golangci-lint run` + `tsc --noEmit` + `next lint` |
| `make test-integration` | Docker Compose integration tests |
| `make docker-build` | `docker buildx build .` |

---

## Development Workflow

1. Read the relevant `aspec/` files before writing any code.
2. Work items live in `aspec/work-items/`. Plans go in `aspec/work-items/plans/`.
3. Follow the workflow in `aspec/workflows/implement-workitem.md`: **plan → implement → tests → docs → review**.
4. Use slash commands: `/plan-workitem`, `/implement-workitem`, `/test-workitem`, `/docs-workitem`, `/review-workitem`.
5. All work happens on feature branches; squash-merge to `main`.
6. CI must pass before merge.

---

## Subagents

Three specialist subagents are defined for implementation work:

- **go-backend** (`.claude/agents/go-backend.md`) — controller and CLI Go code
- **frontend** (`.claude/agents/frontend.md`) — Next.js webapp TypeScript/React
- **devops** (`.claude/agents/devops.md`) — Dockerfile, Makefile, NGINX, GitHub Actions

Use these agents for parallelising implementation across components within a work item.

---

## UI Reference

- **Colors:** Primary `#2DD4BF` (teal), Accent `#F59E0B` (amber), Neutrals: Tailwind slate
- **Font:** Geist Sans (UI), Geist Mono (code/technical values)
- **Layout:** Responsive icon grid — 6 cols (1280px+), 4 cols (1024px), 3 cols (mobile)
- **Background:** Time-of-day gradient (orange/red sunrise/sunset, blue/white midday, black/silver night)
- **Accessibility:** WCAG AA, keyboard nav, ARIA labels, `prefers-reduced-motion` support

---

## Notes / Known Inconsistencies

- Go module path uses `github.com/[owner]/oasis` placeholder — update before first real release.
- s6-overlay and golangci-lint versions must be pinned (see work item 0001 edge cases).
