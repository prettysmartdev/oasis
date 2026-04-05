# Continuous Integration and Deployment

Platform: github

## Pipelines:

Build:
- Triggered on every push and pull request
- Build the Go controller binary: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/controller
- Build the Go oasis CLI binary for all target platforms (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64)
- Build the Next.js webapp: npm ci && npm run build (static export)
- Build the Docker image combining all artifacts using the multi-stage Dockerfile
- Run go vet and golangci-lint on all Go code
- Run tsc --noEmit and next lint on the webapp

Test:
- Run Go unit tests on every push: go test ./... with -race flag
- Run Next.js/Jest unit tests on every push: npm test
- Run integration tests on pull requests targeting main: spin up the container via Docker Compose with a mock Tailscale environment and run the integration test suite against the live management API
  - Note: The integration test job is a no-op stub until a future work item wires up real handlers; the job must remain in the pipeline and exit green
- Upload test coverage reports as CI artifacts
- Fail the pipeline if coverage drops below defined thresholds
  - Initial threshold: 10% (skeleton code); raise in subsequent work items as real logic is added

Releases:
- Triggered by pushing a semver tag (e.g. v1.2.3) to main
- Builds multi-arch Docker image (linux/amd64, linux/arm64) and pushes to GitHub Container Registry (ghcr.io)
- Builds CLI binaries for all target platforms and uploads them as GitHub release assets
- Auto-generates release notes from conventional commits since the last tag

Versioning:
- Follow semantic versioning: MAJOR.MINOR.PATCH
- Use conventional commits (feat:, fix:, chore:, docs:, test:, refactor:) for automated changelog generation
- The controller and CLI both embed their version string at build time via -ldflags "-X main.version=$(git describe --tags --always --dirty)"
- The CLI checks its version against the controller's reported version on each command and warns if they are significantly out of sync

Pinned tool versions (update deliberately, not automatically):
- golangci-lint: v1.57.2 (pinned in ci.yml and Makefile GOLANGCI_LINT_VERSION)
- air (live reload): v1.51.0 (pinned in Makefile AIR_VERSION)
- s6-overlay: v3.1.6.2 (pinned in Dockerfile ARG S6_OVERLAY_VERSION)

Publishing:
- Docker image: ghcr.io/[owner]/oasis:[semver-tag] and ghcr.io/[owner]/oasis:latest
- CLI binaries attached as GitHub release assets: oasis_darwin_amd64, oasis_darwin_arm64, oasis_linux_amd64, oasis_linux_arm64 (each as a .tar.gz with the binary and LICENSE)
- Optional Homebrew tap formula updated automatically on each release

Deployment:
- Self-hosted; no automated server-side deployment pipeline
- End-users install and upgrade via the oasis CLI (`oasis init`, `oasis update`)
- The `oasis update` command pulls the new Docker image and does a stop/start of the container, preserving volumes
