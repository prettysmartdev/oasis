# Local Development

Development: local + docker hybrid (Go and Node.js tools run locally for fast iteration; Docker used for full integration testing)
Build tools: make, go (1.22+), npm (Node.js 20+)

## Workflows:

Developer Loop:
- Prerequisites: Go 1.22+, Node.js 20+, Docker (for integration tests), golangci-lint, air (Go live reload)
- After cloning, run `go mod tidy` once to populate go.sum before building
- Run `make install-tools` to install all dev tooling (air, golangci-lint, etc.) into the local Go bin path
- Copy `.env.local.example` to `.env.local` and fill in a Tailscale auth key for development (use a reusable, pre-authorized key from a dev tailnet)
- Run `make dev` to start both the controller (with live reload via air) and the Next.js dev server in parallel — the Makefile coordinates both processes
  - Controller live reload is configured via `.air.toml` at the repo root; modify that file to adjust watched paths or build flags
  - `make dev` performs a pre-flight check for `air` and fails fast with a clear error if it is not installed
- The controller in dev mode serves the management API on 127.0.0.1:04515 (Tailscale connection requires TS_AUTHKEY in .env.local)
- The Next.js dev server runs on localhost:3000
- Build and test the CLI locally with `make build-cli` — the binary outputs to ./bin/oasis and can be run against the local dev controller

Local testing:
- `make test` — runs `go test ./...` (with -race) and `npm test` for all unit tests
- `make lint` — runs golangci-lint on Go code and tsc --noEmit + next lint on the webapp
- `make test-integration` — builds the full Docker image locally and runs the integration test suite via Docker Compose; requires Docker and a dev Tailscale auth key in `.env.local`
- Use `.env.local` for all local secrets; this file is in .gitignore and must never be committed

Version control:
- Main branch is `main`; all work happens on short-lived feature branches
- Use conventional commits: feat:, fix:, chore:, docs:, test:, refactor:
- Pull requests require passing CI checks and a review before merge
- Squash-merge all PRs to main to keep history clean and changelog-friendly

Documentation:
- Update relevant aspec/ files alongside any architectural change — aspec is the living design document
- API changes must be reflected in architecture/apis.md before or alongside the implementation PR
- Keep the top-level README.md current with installation and quickstart instructions
- Go packages must have godoc-style comments on all exported types and functions
- Significant CLI command changes should be reflected in uxui/cli.md
