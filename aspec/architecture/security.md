# Security

## Network Security

Transport:
- Management API: plain HTTP over loopback (127.0.0.1 only); TLS is unnecessary as traffic never leaves the host kernel's loopback interface
- Webapp and public API: HTTPS via Tailscale / tsnet — Tailscale issues TLS certificates for each node automatically; all tailnet traffic is encrypted with WireGuard at the network layer
- NGINX binds exclusively to the Tailscale interface IP; it is never exposed on 0.0.0.0, the host LAN, or the public internet
- The Docker container publishes only the management API port, and only bound to 127.0.0.1 on the host (e.g. 127.0.0.1:04515:04515)

## API Security

Authentication:
- Management API: no application-level authentication; the authorization boundary is host OS access — only processes that can reach 127.0.0.1:04515 are authorized (i.e. the local user)
- Webapp API: Tailscale provides device-level authentication transparently; only devices enrolled in the user's tailnet and permitted by Tailscale ACLs can reach the oasis node
- No application-level sessions, cookies, or tokens are issued or managed by oasis

RBAC:
- Two implicit roles derived from network position:
  - Host (CLI user): full management access via 127.0.0.1 management API — can register/remove apps, change settings, trigger updates
  - Tailnet visitor: read-only dashboard access via the tailnet — can view and navigate to registered apps but cannot call any management endpoints
- Tailscale ACLs can be used to restrict which tailnet nodes can reach the oasis node for even finer access control
- No per-app access control in v1 — all registered apps are visible to all tailnet visitors who can reach oasis

## Secrets Management

- Tailscale auth key (TS_AUTHKEY) is the only secret in the system
- Provided at initial setup via environment variable passed to `docker run` or via `oasis init`
- Never logged, never returned in API responses, never stored in the SQLite database after initial use
- After the controller authenticates with Tailscale, the tsnet state directory (persisted via Docker volume) is used for subsequent starts — the auth key is not required again unless the node is removed from the tailnet
- The oasis CLI does not store the auth key locally; it passes it to the controller's /api/v1/setup endpoint once during init

## Container Security

- Controller and NGINX run as a non-root user (uid 1000) inside the container
- No privileged mode required; tsnet uses userspace WireGuard (no kernel module, no special Linux capabilities)
- Minimal base image (debian:bookworm-slim) to reduce attack surface
- Multi-stage Docker build ensures no build toolchain or source code is present in the runtime image
- Two Docker volumes are required: one for SQLite database persistence, one for Tailscale tsnet state — both should be owned by uid 1000
