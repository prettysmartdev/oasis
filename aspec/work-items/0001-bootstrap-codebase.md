# Work Item: Task

Title: Bootstrap codebase
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary:
- Create the foundational repository structure for oasis with skeleton packages for the controller/gateway, CLI, and Next.js webapp — no logic implemented, just scaffolding.
- Set up the Makefile, multi-stage Dockerfile, `.env.local.example`, and GitHub Actions workflows so the repo builds, lints, and tests cleanly from day one.
- Establishes the build and CI foundation so all future work items can be merged against a consistently green pipeline.

## User Stories

### User Story 1:
As a: Owner / Admin

I want to:
clone the oasis repo and run `make dev` to start both the controller and the Next.js dev server locally

So I can:
begin developing features against a running skeleton immediately without any manual environment setup

### User Story 2 (if needed):
As a: Owner / Admin

I want to:
open a pull request and see the full GitHub Actions CI pipeline pass (build, lint, unit tests) automatically

So I can:
trust that the build and test infrastructure is working before any real logic is written, and maintain that confidence as the project grows

### User Story 3 (if needed):
As a: Owner / Admin

I want to:
run `make build-cli` and get a working `./bin/oasis` binary that prints a version string and top-level help

So I can:
verify the CLI scaffold is wired up correctly and begin implementing commands against it


## Implementation Details:

### Repository layout
```
oasis/
├── cmd/
│   ├── controller/         # Go: main entry point for the controller binary
│   │   └── main.go         # stub: starts two empty http.ServeMux servers (mgmt + tsnet)
│   └── oasis/              # Go: main entry point for the CLI binary
│       └── main.go         # stub: calls root cobra command
├── internal/
│   ├── controller/         # Go: controller domain packages (empty stubs)
│   │   ├── api/            # management API handler skeletons
│   │   ├── db/             # SQLite store skeleton
│   │   ├── nginx/          # NGINX config generation skeleton
│   │   └── tsnet/          # Tailscale/tsnet integration skeleton
│   └── cli/                # Go: CLI command implementations (empty stubs)
│       ├── root.go         # cobra root command with global flags
│       ├── app.go          # `oasis app` subcommand group stub
│       └── settings.go     # `oasis settings` subcommand group stub
├── webapp/                 # Next.js app (App Router)
│   ├── app/
│   │   └── page.tsx        # stub: renders "oasis" heading
│   ├── components/         # empty; shadcn/ui components will live here
│   ├── package.json
│   ├── tsconfig.json
│   └── next.config.js      # output: 'export' for static build
├── .github/
│   └── workflows/
│       ├── ci.yml          # build + lint + unit test on push/PR
│       └── release.yml     # build multi-arch image + CLI binaries on semver tag
├── Dockerfile              # multi-stage: node:20-alpine → golang:1.22-alpine → debian:bookworm-slim
├── docker-compose.dev.yml  # integration test compose file (stubbed)
├── Makefile                # dev, build, test, lint, build-cli, install-tools targets
├── .env.local.example      # TS_AUTHKEY=, OASIS_MGMT_PORT=04515
├── .gitignore
└── go.mod / go.sum
```

### Go module
- Module path: `github.com/[owner]/oasis`
- Initial dependencies: `cobra`, `modernc.org/sqlite`, `tailscale.com/tsnet` (stub import only), `github.com/google/uuid`
- Build constraint: `CGO_ENABLED=0` enforced in Makefile and CI; use `modernc.org/sqlite` (pure-Go driver)
- Embed version via `-ldflags "-X main.version=$(git describe --tags --always)"`

### Controller skeleton (`cmd/controller/main.go`)
- Parse config from env vars (`OASIS_MGMT_PORT`, defaulting to `04515`)
- Start two empty `http.ServeMux` listeners: management API on `127.0.0.1:$PORT`, tsnet API stub
- Log startup message including version and bound address; exit cleanly on `SIGINT`/`SIGTERM`
- No actual handlers yet — all routes return `501 Not Implemented`

### CLI skeleton (`cmd/oasis/main.go` + `internal/cli/`)
- Root cobra command with `--config`, `--json`, `--quiet`, `--version` global flags (see uxui/cli.md)
- Subcommand groups stubbed: `app`, `settings`, `init`, `start`, `stop`, `restart`, `status`, `update`, `logs`, `db`
- Each subcommand prints `"not yet implemented"` and exits 0
- `--version` flag prints the embedded version string

### Next.js webapp skeleton (`webapp/`)
- `next.config.js` with `output: 'export'` and `distDir: '../dist/webapp'`
- App Router `app/layout.tsx` with Tailwind globals and `app/page.tsx` rendering a placeholder heading
- `package.json` with scripts: `dev`, `build`, `lint`, `test`
- Tailwind CSS + shadcn/ui initialised (via `shadcn-ui init` with a neutral theme)
- Jest + `@testing-library/react` configured for unit tests; one smoke test confirming the page renders

### Makefile targets
- `install-tools` — installs `air`, `golangci-lint` via `go install`
- `dev` — runs controller with `air` and `next dev` in parallel (using `make -j2`)
- `build` — builds controller and CLI binaries (`./bin/controller`, `./bin/oasis`)
- `build-cli` — builds CLI only; outputs `./bin/oasis`
- `test` — `go test -race ./...` + `npm --prefix webapp test -- --ci`
- `lint` — `golangci-lint run ./...` + `npm --prefix webapp run lint`
- `test-integration` — `docker compose -f docker-compose.dev.yml up --abort-on-container-exit`
- `docker-build` — `docker buildx build .`

### Dockerfile (multi-stage)
- Stage 1 (`node:20-alpine`): `npm ci && npm run build` → `/dist/webapp`
- Stage 2 (`golang:1.22-alpine`): `CGO_ENABLED=0 go build` for controller and CLI binaries
- Stage 3 (`debian:bookworm-slim`): copy binaries + static assets; install NGINX and s6-overlay; set `USER 1000`
- No Tailscale daemon — tsnet operates in userspace; no `CAP_NET_ADMIN` needed

### GitHub Actions workflows
- **`ci.yml`** (trigger: push + pull_request):
  - Job `lint-go`: `golangci-lint run ./...` + `go vet ./...`
  - Job `lint-web`: `tsc --noEmit` + `next lint`
  - Job `test-go`: `go test -race -coverprofile=coverage.out ./...`; upload coverage artifact
  - Job `test-web`: `npm test -- --ci --coverage`; upload coverage artifact
  - Job `build`: build all Go binaries + Next.js static export + Docker image (no push)
- **`release.yml`** (trigger: push tag `v*`):
  - Build multi-arch Docker image (`linux/amd64`, `linux/arm64`) and push to `ghcr.io`
  - Build CLI binaries for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`; archive as `.tar.gz`
  - Create GitHub release with auto-generated notes and attach CLI archives

- See devops/cicd.md for full pipeline requirements


## Edge Case Considerations:
- **Module path placeholder**: the `go.mod` module path uses `github.com/[owner]/oasis`; document in a `// TODO` comment that this must be updated before first real release or when the GitHub org/repo is confirmed
- **`air` not installed**: `make dev` should fail fast with a clear message if `air` is not found; add a pre-flight check or point to `make install-tools`
- **Port conflict on dev startup**: if `04515` is already in use, the controller skeleton should log a clear bind error and exit non-zero rather than silently failing
- **Next.js static export limitations**: App Router with `output: 'export'` does not support dynamic routes or server actions; ensure no such patterns are introduced in the skeleton that would break the build
- **CGO_ENABLED=0 with sqlite**: `modernc.org/sqlite` must be used (not `mattn/go-sqlite3`) to allow a fully static build; verify this in CI with `file ./bin/controller | grep 'statically linked'`
- **`.env.local` committed by accident**: `.gitignore` must include `.env.local`; CI should never require a real `TS_AUTHKEY` for build or unit test jobs
- **s6-overlay version pinning**: pin the s6-overlay version in the Dockerfile to avoid unexpected breakage on image rebuilds
- **golangci-lint version**: pin golangci-lint version in `ci.yml` and `install-tools` to avoid lint rule drift across contributors


## Test Considerations:
- **Unit tests (Go)**: one smoke test per skeleton package confirming the package compiles and exported types/functions exist; e.g. `TestControllerStartup` that confirms the server starts and returns `501` on an unknown route
- **Unit tests (web)**: one Jest smoke test confirming the home page renders without throwing; confirms Tailwind and shadcn/ui are wired correctly
- **CLI flag test**: `TestRootCmd` confirming `--version` prints a non-empty string and exits 0; `TestSubcommandNotImplemented` for one representative subcommand
- **Build verification**: CI `build` job verifies that `go build ./cmd/controller` and `go build ./cmd/oasis` both succeed with `CGO_ENABLED=0`; static linkage check for the controller binary
- **No integration tests in this work item**: docker-compose.dev.yml is a stub; the integration test job in CI should be skipped or a no-op until a subsequent work item wires up real handlers
- **Coverage thresholds**: set initial thresholds low (e.g. 10%) to avoid blocking CI on skeleton code; raise thresholds in subsequent work items as real logic is added


## Codebase Integration:
- Follow established conventions, best practices, testing, and architecture patterns from the project's aspec
- Management API changes must follow the REST conventions in architecture/apis.md
- New CLI commands must follow the command/flag structure in uxui/cli.md
- Controller changes must not break graceful NGINX reload or restart safety guarantees
- The management API must only ever bind to `127.0.0.1` — enforced in the skeleton and verified in unit tests (see foundation.md)
- All Go packages must have godoc-style package comments from the start; this avoids a documentation debt cleanup pass later (see devops/localdev.md)
- The `webapp/` directory name and Next.js `distDir` path must align with the paths referenced in the Dockerfile's `COPY` instructions — agree on these paths in this work item and do not change them without updating both
- The `.env.local.example` file establishes the canonical list of environment variables consumed by the system; keep it in sync as new env vars are added in future work items (see devops/localdev.md)
