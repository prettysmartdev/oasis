# Project Foundation

Name: oasis
Type: saas (self-hosted)
Purpose: A homescreen for all your vibe-coded apps and agents — a self-hosted dashboard that discovers, organizes, and exposes locally-running apps and AI agents exclusively over the user's Tailscale network (tailnet).

# Technical Foundation

## Languages and Frameworks

### Frontend (Webapp)
Language: TypeScript
Frameworks: Next.js (App Router), Tailwind CSS, shadcn/ui
Guidance:
- Use Next.js App Router with React Server Components where possible
- Keep client components minimal; prefer server-side data fetching
- Use shadcn/ui component primitives for consistent, accessible UI
- The built Next.js output is compiled to a static export and served by NGINX inside the Docker container; no Next.js runtime runs in the container

### Backend (Controller)
Language: Go
Frameworks: net/http (stdlib), tsnet (Tailscale Go SDK), crossplane-go (NGINX config generation), modernc.org/sqlite (CGO-free SQLite driver)
Guidance:
- The controller is the single source of truth for the app/agent registry, NGINX configuration, and Tailscale connectivity
- Exposes two HTTP servers: a local-only management API bound to 127.0.0.1, and an internal API accessible over the tsnet interface for the webapp
- Uses tsnet to join the user's tailnet as a named node (e.g. "oasis") without requiring the Tailscale daemon on the host
- Generates and hot-reloads NGINX config via crossplane-go whenever the app registry changes
- Stores all state in a local SQLite database for simple, zero-dependency persistence
- Must compile to a static binary (CGO_ENABLED=0) for inclusion in the Docker image

### CLI (oasis)
Language: Go
Frameworks: cobra (CLI framework), net/http (stdlib, for management API calls)
Guidance:
- Compiles to a single static binary named `oasis` for distribution on the host machine
- Communicates exclusively with the controller's local management API over localhost
- Provides human-friendly output by default; supports a --json flag for machine-readable output
- No runtime dependencies; distributed as a standalone binary

# Best Practices
- Organize code in small, simple, modular packages with clear responsibilities and minimal coupling
- Each package should have unit tests that validate behaviour in terms of inputs and outputs
- Integration tests should cover: controller <-> NGINX config generation, controller <-> Tailscale registration via tsnet, and CLI <-> management API interactions
- The controller must be safe to restart without losing state (all state in SQLite in a mounted volume; Tailscale state persisted to a mounted volume)
- NGINX config reloads must be graceful (SIGHUP) to avoid dropped connections during route changes
- All management API endpoints must be bound exclusively to 127.0.0.1, never 0.0.0.0
- Secrets (Tailscale auth key) must never be logged or returned in API responses

# Personas

### Persona 1:
Name: The Vibe Coder (Owner / Admin)
Purpose: The person who runs oasis on their machine and manages the apps and agents registered with it
Use-cases:
- Install oasis via the CLI and Docker
- Register locally-running apps and AI agents with the dashboard
- Access the personal dashboard over their tailnet from any device
- Manage app routes, display names, icons, and enable/disable state via the CLI
- Update oasis, change settings, and troubleshoot via CLI commands
RBAC:
- Full administrative control over all settings and registered apps
- Access is implicit: only processes on the host machine can reach the local management API
- The tailnet (Tailscale) provides the outer authentication boundary for the dashboard

### Persona 2:
Name: Tailnet Visitor (Read-only Consumer)
Purpose: A trusted device or person on the user's tailnet who can view and navigate to the registered apps via the dashboard
Use-cases:
- Browse the oasis dashboard from their tailnet-connected device
- Navigate to registered apps and agents via the dashboard links
RBAC:
- Read-only access to the dashboard and app links
- Cannot modify settings, register apps, or reach the management API
- Access is scoped to whatever the user's Tailscale ACLs permit
