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
- App: { id (uuid), name (string), slug (string, URL-safe), upstreamURL (string), displayName (string), description (string), icon (string, URL or emoji), tags ([]string), enabled (bool), health ("healthy"|"unreachable"|"unknown"), createdAt (RFC3339), updatedAt (RFC3339) }
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

### Settings (management API only)
- GET   /api/v1/settings        — get current settings (authKey omitted)
- PATCH /api/v1/settings        — update settings

### Setup (management API only, one-time)
- POST /api/v1/setup            — provide initial Tailscale auth key and hostname; triggers tsnet join
