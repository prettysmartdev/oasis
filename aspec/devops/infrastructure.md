# Infrastructure

Deployment platform: docker (single container, self-hosted on the user's machine or home server)
Cloud platform: none (fully self-hosted; Tailscale is the only external network dependency)
Automation: Makefile (build, test, lint, run, and release tasks)

## Architecture:

Best practices:
- Single Docker container contains the controller (Go binary), NGINX, and the built Next.js static assets
- Multi-stage Dockerfile: stage 1 builds the Next.js webapp (node:20-alpine), stage 2 builds the Go binaries (golang:1.22-alpine, CGO_ENABLED=0), stage 3 assembles the final runtime image (debian:bookworm-slim)
- No build toolchain or source code is present in the runtime image
- NGINX and the controller are managed by a lightweight process supervisor (s6-overlay) inside the container so both processes restart cleanly if they crash
- Tailscale tsnet state is persisted via a named Docker volume (oasis-ts-state) so the node does not re-authenticate on container restart
- The SQLite database is persisted via a named Docker volume (oasis-db)
- The management API port is published bound to the host loopback only: 127.0.0.1:04515:04515 — never 0.0.0.0
- No host network mode is required; tsnet operates entirely in userspace (no kernel Tailscale daemon needed)
- The container image is versioned and pinned by digest in production use; `oasis update` pulls by explicit tag, not `latest`

Security and RBAC:
- All processes inside the container run as uid 1000 (non-root); the Docker USER directive enforces this
- No privileged mode; no special Linux capabilities (CAP_NET_ADMIN etc.) required — tsnet uses userspace WireGuard
- The management API port (04515) is always published as 127.0.0.1:PORT:PORT, never 0.0.0.0:PORT:PORT
- TS_AUTHKEY is passed as a Docker environment variable at first run only; the volume-persisted tsnet state removes the need for it on subsequent starts
- Docker volumes for tsnet state and the database should be backed up periodically; `oasis db backup` facilitates this
- No inbound ports are opened on the host firewall for the tailnet interface; tsnet establishes outbound WireGuard tunnels
