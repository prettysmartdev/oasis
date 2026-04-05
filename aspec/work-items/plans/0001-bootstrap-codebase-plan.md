# Plan: 0001 — Bootstrap Codebase

Work Item: `aspec/work-items/0001-bootstrap-codebase.md`
Status: Ready to implement

---

## Overview

Stand up the complete repository skeleton from scratch. No business logic is implemented — every package is a well-structured stub that compiles cleanly, passes linting, and has at least one smoke test. The goal is a consistently green CI pipeline that all future work items merge against.

---

## Open Questions / Decisions Made

1. **Management API port** — `04515` (confirmed via CLAUDE.md update; this is canonical).
2. **Go module path** — `github.com/[owner]/oasis` with a `// TODO: update before first release` comment in `go.mod`.
3. **`webapp/distDir`** — `'../dist/webapp'` (must align with Dockerfile COPY paths; locking this in now).
4. **s6-overlay version** — must be pinned; choose latest stable at time of implementation (e.g. `v3.1.6.2`).
5. **golangci-lint version** — must be pinned in CI and `install-tools` (e.g. `v1.57.2`).
6. **air version** — pinned in `install-tools` for reproducibility.
7. **Integration test CI job** — stubbed/no-op in this work item; real integration tests come in a future work item.
8. **Coverage threshold** — 10% initial floor to avoid blocking CI on skeleton code.

---

## Implementation Steps

### Phase 1: Repository Foundation
**Owner:** devops
**Files:** `go.mod`, `go.sum`, `.gitignore`, `.env.local.example`

#### Step 1.1 — `go.mod`
```
module github.com/[owner]/oasis // TODO: update owner before first release

go 1.22
```
Initial dependencies to `require`:
- `github.com/spf13/cobra`
- `modernc.org/sqlite`
- `tailscale.com/tsnet`
- `github.com/google/uuid`

Add a `// TODO: update module path before first release` comment above the module line.

#### Step 1.2 — `.gitignore`
Must include:
```
.env.local          # CRITICAL: never commit secrets
bin/
dist/
webapp/.next/
webapp/node_modules/
*.db
*.db.bak
coverage.out
coverage/
```

#### Step 1.3 — `.env.local.example`
Canonical env var list (establish now; keep in sync as vars are added):
```
# Tailscale auth key — obtain from https://login.tailscale.com/admin/settings/keys
# Use a reusable, pre-authorized key for dev. Never commit .env.local.
TS_AUTHKEY=

# Management API port (default: 04515)
OASIS_MGMT_PORT=04515

# Tailscale node hostname (default: oasis)
OASIS_HOSTNAME=oasis

# SQLite database path inside container (default: /data/db/oasis.db)
OASIS_DB_PATH=/data/db/oasis.db

# Tailscale tsnet state dir inside container (default: /data/ts-state)
OASIS_TS_STATE_DIR=/data/ts-state

# Log verbosity: info | debug | warn | error (default: info)
OASIS_LOG_LEVEL=info
```

---

### Phase 2: Go Package Stubs
**Owner:** go-backend
**Files:** all `cmd/` and `internal/` Go files

#### Step 2.1 — `cmd/controller/main.go`
- `// Package main is the entry point for the oasis controller binary.` package comment
- Parse `OASIS_MGMT_PORT` from env (default `04515`)
- Create two empty `http.ServeMux` instances: management mux and tsnet mux
- Register a catch-all handler on both that returns `501 Not Implemented` with JSON body `{"error":"not implemented","code":"NOT_IMPLEMENTED"}`
- Listen on `127.0.0.1:$PORT` for management API — **hard-code the loopback host, never use a variable for the bind address**
- Log startup: `oasis controller version=<version> mgmt=127.0.0.1:<port>`
- Handle `SIGINT`/`SIGTERM` for clean shutdown using `context` + `os/signal`
- Embed version via `var version = "dev"` (overridden by `-ldflags` at build time)
- If the management port is already in use, log a clear error (`bind: address already in use on 127.0.0.1:<port> — is another instance running? Change OASIS_MGMT_PORT or run 'oasis stop'.`) and `os.Exit(1)`

#### Step 2.2 — `internal/controller/api/` package stub
- `// Package api implements the oasis management HTTP API handlers.` package comment
- Empty `handler.go` with a single exported struct `Handler` and constructor `func New() *Handler`
- No routes yet

#### Step 2.3 — `internal/controller/db/` package stub
- `// Package db provides SQLite persistence for the oasis app registry and settings.` package comment
- Empty `store.go` with exported struct `Store` and constructor `func New(path string) (*Store, error)` (returns nil, nil for now)

#### Step 2.4 — `internal/controller/nginx/` package stub
- `// Package nginx generates and applies NGINX configuration for the oasis gateway.` package comment
- Empty `config.go` with exported struct `Configurator` and constructor `func New() *Configurator`

#### Step 2.5 — `internal/controller/tsnet/` package stub
- `// Package tsnet manages the Tailscale tsnet node for the oasis controller.` package comment
- Empty `node.go` with exported struct `Node` and constructor `func New() *Node`
- Import `tailscale.com/tsnet` with a blank import or minimal stub to ensure the dependency is recorded in `go.sum`

#### Step 2.6 — `cmd/oasis/main.go`
- `// Package main is the entry point for the oasis CLI binary.` package comment
- Single call to `cli.Execute()`
- Embed version: `var version = "dev"`; pass to root command

#### Step 2.7 — `internal/cli/root.go`
- `// Package cli implements the oasis command-line interface.` package comment
- Cobra `rootCmd` with:
  - Use: `oasis`
  - Short: `oasis — manage your self-hosted app dashboard`
  - Global persistent flags: `--config string` (default `~/.oasis/config.json`), `--json` (bool), `--quiet` (bool)
  - `--version` flag that prints the embedded version and exits 0 (use cobra's built-in version support)
- Exported `Execute()` function
- All subcommands registered in `init()` via `rootCmd.AddCommand(...)`

#### Step 2.8 — `internal/cli/app.go`
- `appCmd` cobra command group (`oasis app`)
- Subcommands stubbed: `add`, `list`, `show`, `remove`, `enable`, `disable`, `update`
- Each `RunE` prints `"not yet implemented\n"` to stdout and returns nil

#### Step 2.9 — `internal/cli/settings.go`
- `settingsCmd` cobra command group (`oasis settings`)
- Subcommands stubbed: `get`, `set`
- Each `RunE` prints `"not yet implemented\n"` and returns nil

#### Step 2.10 — Remaining top-level CLI stubs
Add these directly to `root.go` `init()` or as separate files:
- `init`, `start`, `stop`, `restart`, `status`, `update`, `logs`, `db` — each a minimal cobra command that prints `"not yet implemented\n"`

---

### Phase 3: Next.js Webapp Skeleton
**Owner:** frontend
**Files:** all `webapp/` files

#### Step 3.1 — `webapp/package.json`
Scripts:
```json
{
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "lint": "next lint",
    "test": "jest --passWithNoTests"
  }
}
```
Dependencies: `next`, `react`, `react-dom`
Dev dependencies: `typescript`, `@types/react`, `@types/node`, `tailwindcss`, `postcss`, `autoprefixer`, `jest`, `jest-environment-jsdom`, `@testing-library/react`, `@testing-library/jest-dom`
shadcn/ui initialised with neutral theme (run `npx shadcn-ui init` equivalent configuration manually in config files).

#### Step 3.2 — `webapp/next.config.js`
```js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  distDir: '../dist/webapp',
}
module.exports = nextConfig
```

#### Step 3.3 — `webapp/tsconfig.json`
Standard Next.js App Router tsconfig with `strict: true`.

#### Step 3.4 — `webapp/tailwind.config.js` + `webapp/postcss.config.js`
Standard Tailwind CSS config with content paths pointing to `app/**/*.{ts,tsx}` and `components/**/*.{ts,tsx}`.

#### Step 3.5 — `webapp/app/layout.tsx`
- Import Tailwind globals CSS
- `<html lang="en">` with dark mode class support
- Geist Sans + Geist Mono font configuration via `next/font`

#### Step 3.6 — `webapp/app/globals.css`
Tailwind directives: `@tailwind base`, `@tailwind components`, `@tailwind utilities`.

#### Step 3.7 — `webapp/app/page.tsx`
Minimal stub:
```tsx
export default function HomePage() {
  return (
    <main>
      <h1>oasis</h1>
    </main>
  )
}
```

#### Step 3.8 — `webapp/components/` directory
Create `.gitkeep` — shadcn/ui components will live here.

#### Step 3.9 — `webapp/jest.config.js` + `webapp/jest.setup.ts`
Configure Jest with `jest-environment-jsdom`; import `@testing-library/jest-dom` in setup file.

---

### Phase 4: Makefile
**Owner:** devops
**File:** `Makefile`

```makefile
BINARY_CONTROLLER := ./bin/controller
BINARY_CLI        := ./bin/oasis
VERSION           := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS           := -ldflags "-X main.version=$(VERSION)"
GOLANGCI_LINT_VERSION := v1.57.2
AIR_VERSION            := v1.51.0

.PHONY: install-tools dev build build-cli test lint test-integration docker-build

install-tools:
	@echo "Installing dev tools..."
	go install github.com/air-verse/air@$(AIR_VERSION)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

dev:
	@command -v air >/dev/null 2>&1 || { \
		echo "ERROR: 'air' not found. Run 'make install-tools' first."; exit 1; }
	make -j2 _dev-controller _dev-web

_dev-controller:
	air -c .air.toml

_dev-web:
	npm --prefix webapp run dev

build: $(BINARY_CONTROLLER) $(BINARY_CLI)

$(BINARY_CONTROLLER):
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_CONTROLLER) ./cmd/controller

$(BINARY_CLI):
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_CLI) ./cmd/oasis

build-cli: $(BINARY_CLI)

test:
	go test -race ./...
	npm --prefix webapp test -- --ci

lint:
	golangci-lint run ./...
	npm --prefix webapp run lint

test-integration:
	docker compose -f docker-compose.dev.yml up --abort-on-container-exit

docker-build:
	docker buildx build .
```

Also create `.air.toml` for the controller live-reload configuration (points to `cmd/controller`, outputs to `tmp/controller`).

---

### Phase 5: Dockerfile (multi-stage)
**Owner:** devops
**File:** `Dockerfile`

Three stages:

**Stage 1 — `node:20-alpine` (webapp builder)**
```dockerfile
FROM node:20-alpine AS webapp-builder
WORKDIR /build
COPY webapp/package*.json ./webapp/
RUN npm --prefix webapp ci
COPY webapp/ ./webapp/
RUN npm --prefix webapp run build
# Output: /build/dist/webapp (matches distDir: '../dist/webapp')
```

**Stage 2 — `golang:1.22-alpine` (go builder)**
```dockerfile
FROM golang:1.22-alpine AS go-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.version=${VERSION}" \
    -o bin/controller ./cmd/controller
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.version=${VERSION}" \
    -o bin/oasis ./cmd/oasis
```

**Stage 3 — `debian:bookworm-slim` (runtime)**
```dockerfile
FROM debian:bookworm-slim AS runtime
ARG S6_OVERLAY_VERSION=3.1.6.2
# Install NGINX, curl, s6-overlay
RUN apt-get update && apt-get install -y --no-install-recommends \
    nginx curl xz-utils && rm -rf /var/lib/apt/lists/*
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp/
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-x86_64.tar.xz /tmp/
RUN tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz \
 && tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz \
 && rm /tmp/*.tar.xz
COPY --from=go-builder /build/bin/controller /usr/local/bin/controller
COPY --from=go-builder /build/bin/oasis /usr/local/bin/oasis
COPY --from=webapp-builder /build/dist/webapp /srv/webapp
RUN groupadd -g 1000 oasis && useradd -u 1000 -g oasis -s /sbin/nologin oasis
USER 1000
EXPOSE 04515
ENTRYPOINT ["/init"]
```

Note: s6-overlay service definitions (for controller and NGINX) are stub files only in this work item — full wiring comes in a later work item.

---

### Phase 6: `docker-compose.dev.yml`
**Owner:** devops
**File:** `docker-compose.dev.yml`

Stub compose file for integration tests. In this work item it defines the service but the test job in CI is a no-op:
```yaml
services:
  oasis:
    build: .
    ports:
      - "127.0.0.1:04515:04515"
    volumes:
      - oasis-db:/data/db
      - oasis-ts-state:/data/ts-state
    environment:
      - OASIS_MGMT_PORT=04515

volumes:
  oasis-db:
  oasis-ts-state:
```

---

### Phase 7: GitHub Actions Workflows
**Owner:** devops
**Files:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`

#### `ci.yml` — trigger: `push` + `pull_request`

Jobs (all run on `ubuntu-latest`):

**`lint-go`**
- Checkout, setup Go 1.22
- Run `golangci-lint run ./...` (pinned version via `golangci-lint-action`)
- Run `go vet ./...`

**`lint-web`**
- Checkout, setup Node 20
- `npm --prefix webapp ci`
- `npx --prefix webapp tsc --noEmit`
- `npm --prefix webapp run lint`

**`test-go`**
- Checkout, setup Go 1.22
- `go test -race -coverprofile=coverage.out ./...`
- Upload `coverage.out` as artifact
- Fail if coverage < 10%

**`test-web`**
- Checkout, setup Node 20
- `npm --prefix webapp ci`
- `npm --prefix webapp test -- --ci --coverage`
- Upload coverage artifact

**`build`**
- Checkout, setup Go 1.22 + Node 20
- `CGO_ENABLED=0 go build ./cmd/controller` + `go build ./cmd/oasis`
- `file ./bin/controller | grep 'statically linked'` — static linkage assertion
- `npm --prefix webapp ci && npm --prefix webapp run build`
- `docker buildx build .` (no push)

**`test-integration`** — **no-op / skipped in this work item**
- Add a step that echoes `"Integration tests not yet implemented (work item 0001 stub)"` and exits 0
- This keeps the job green without requiring Docker or a real Tailscale key

#### `release.yml` — trigger: `push` tags matching `v*`

Jobs:

**`docker`**
- Set up QEMU + buildx for multi-arch
- Build and push `ghcr.io/[owner]/oasis:$TAG` and `:latest` for `linux/amd64,linux/arm64`
- Pass `VERSION=$TAG` build arg for `ldflags`

**`cli-binaries`**
- Matrix: `{os: darwin, arch: amd64}`, `{darwin, arm64}`, `{linux, amd64}`, `{linux, arm64}`
- `CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags "..." -o oasis ./cmd/oasis`
- Archive: `tar -czf oasis_${os}_${arch}.tar.gz oasis LICENSE`
- Upload as release assets

**`release`**
- `softprops/action-gh-release` with auto-generated notes from conventional commits
- Attach all CLI archives

---

### Phase 8: Unit Tests
**Owner:** go-backend (Go), frontend (web)

#### Go tests

**`internal/controller/api/handler_test.go`**
- `TestHandlerNew` — `New()` returns a non-nil `*Handler`

**`internal/controller/db/store_test.go`**
- `TestStoreNew` — `New("")` returns without panic

**`internal/controller/nginx/config_test.go`**
- `TestConfiguratorNew` — `New()` returns non-nil `*Configurator`

**`internal/controller/tsnet/node_test.go`**
- `TestNodeNew` — `New()` returns non-nil `*Node`

**`cmd/controller/main_test.go`** (integration-style unit test)
- `TestControllerLoopback` — starts the management server on a random free port, sends a request, confirms `501` response and `Content-Type: application/json`
- `TestControllerBindAddress` — asserts the listener address starts with `127.0.0.1:` (never `0.0.0.0`)

**`internal/cli/root_test.go`**
- `TestRootCmdVersion` — executes `rootCmd` with `--version`, confirms output is non-empty and exit code 0
- `TestSubcommandNotImplemented` — executes `oasis app list`, confirms output contains `"not yet implemented"` and exit code 0

#### Web tests

**`webapp/__tests__/page.test.tsx`**
- `renders home page without throwing` — renders `<HomePage />` with React Testing Library; confirms `<h1>` with text "oasis" is present

---

### Phase 9: `.golangci.yml`
**Owner:** go-backend / devops

Create a minimal `.golangci.yml` at the repo root:
```yaml
run:
  timeout: 5m

linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

---

## File Creation Checklist

```
go.mod
go.sum                          (generated by go mod tidy)
.gitignore
.env.local.example
.golangci.yml
.air.toml
Makefile
Dockerfile
docker-compose.dev.yml
cmd/
  controller/
    main.go
    main_test.go
  oasis/
    main.go
internal/
  controller/
    api/
      handler.go
      handler_test.go
    db/
      store.go
      store_test.go
    nginx/
      config.go
      config_test.go
    tsnet/
      node.go
      node_test.go
  cli/
    root.go
    root_test.go
    app.go
    settings.go
webapp/
  package.json
  package-lock.json             (generated by npm install)
  next.config.js
  tsconfig.json
  tailwind.config.js
  postcss.config.js
  jest.config.js
  jest.setup.ts
  app/
    layout.tsx
    page.tsx
    globals.css
  components/
    .gitkeep
  __tests__/
    page.test.tsx
.github/
  workflows/
    ci.yml
    release.yml
```

---

## Edge Case Handling Plan

| Edge Case | Handling |
|---|---|
| `air` not installed | `make dev` runs `command -v air` pre-flight; prints clear error + `make install-tools` hint |
| Port already in use (04515) | Controller logs specific error with port and hint; exits non-zero |
| `.env.local` accidentally committed | `.gitignore` entry; CI `build` job verified to not need `TS_AUTHKEY` |
| `CGO_ENABLED=0` broken | CI `build` job runs `file ./bin/controller | grep 'statically linked'` assertion |
| Next.js dynamic route introduced | `output: 'export'` causes `next build` to fail immediately — self-enforcing |
| `go.mod` module path placeholder | `// TODO` comment in `go.mod`; noted in README |
| s6-overlay version drift | Version pinned in Dockerfile `ARG S6_OVERLAY_VERSION=3.1.6.2` |
| golangci-lint version drift | Pinned in `ci.yml` action input and `Makefile` `GOLANGCI_LINT_VERSION` variable |
| `distDir` path mismatch with Dockerfile | Both set to `../dist/webapp` → `/build/dist/webapp`; verified by Docker build step in CI |

---

## Subagent Assignment Summary

| Phase | Owner | Parallelisable? |
|---|---|---|
| 1 — Repo foundation | devops | Yes — independent of Go and web |
| 2 — Go stubs | go-backend | Yes — after Phase 1 |
| 3 — Next.js skeleton | frontend | Yes — parallel to Phase 2 |
| 4 — Makefile | devops | After Phases 2 & 3 (needs to know targets) |
| 5 — Dockerfile | devops | After Phases 2 & 3 |
| 6 — docker-compose | devops | Parallel to Phase 5 |
| 7 — GitHub Actions | devops | After Phases 4 & 5 |
| 8 — Unit tests | go-backend + frontend | After Phases 2 & 3 |
| 9 — golangci config | devops / go-backend | Parallel to Phase 7 |

Phases 2, 3, and 1 can all be started in parallel. Phases 4–7 depend on 2 and 3 being structurally defined (file paths, binary names). Phase 8 runs after 2 and 3.

---

## Acceptance Criteria

- [ ] `go build ./cmd/controller` and `go build ./cmd/oasis` succeed with `CGO_ENABLED=0`
- [ ] `file ./bin/controller` reports statically linked
- [ ] `go test -race ./...` passes
- [ ] `npm --prefix webapp run build` produces a static export in `dist/webapp/`
- [ ] `npm --prefix webapp test -- --ci` passes
- [ ] `make lint` exits 0
- [ ] `make build-cli` produces `./bin/oasis`; running `./bin/oasis --version` prints a non-empty string
- [ ] `./bin/oasis app list` prints `"not yet implemented"` and exits 0
- [ ] Controller binary binds management API to `127.0.0.1:04515` only
- [ ] CI pipeline (`ci.yml`) is fully green on push
- [ ] `.env.local` is in `.gitignore`
- [ ] `docker buildx build .` succeeds (no push)
