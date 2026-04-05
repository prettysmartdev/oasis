# Experience

## Setup and Installation (no account required — fully self-hosted)

Signup flow:
- No signup or account creation required; oasis is entirely self-hosted
- The user installs the oasis CLI binary (from GitHub releases or Homebrew) on their host machine
- The user runs `oasis init`, which walks them through:
  1. Providing a Tailscale auth key (the CLI prints a direct link to the Tailscale admin key creation page)
  2. Choosing a Tailscale hostname (default: "oasis" — will be reachable as oasis.[tailnet-name].ts.net)
  3. Optionally customizing the management port (default: 04515)
  4. Pulling the Docker image and starting the container
  5. Waiting for Tailscale to connect and confirming the dashboard is reachable
- After init completes the CLI prints the dashboard URL and the user can open it immediately from any tailnet device

Account management:
- No oasis-level accounts; user identity is managed entirely by Tailscale
- The Tailscale admin console is used to manage which devices are on the tailnet and which ACLs apply
- The oasis hostname can be changed at any time via `oasis settings set tailscale-hostname <new-name>` (triggers a container restart)

Invitations and team/group management:
- Access for other tailnet members is controlled via Tailscale ACLs — no oasis-level invite flow exists
- Tailscale's node sharing feature can be used to share the oasis node with users outside the tailnet (e.g. family members, collaborators)
- All tailnet visitors who can reach the oasis node see the same dashboard (no per-user views in v1)

RBAC/permissions:
- Host machine user: full admin — CLI access and full management API
- Tailnet devices: read-only dashboard access
- No fine-grained per-app visibility controls in v1; all registered apps are visible to all tailnet visitors who can reach the node

Billing, subscriptions, plans:
- Free and open source; no billing, subscriptions, or plans

## Regular usage

Login flow:
- No login to oasis; the dashboard is available to any device on the user's tailnet without authentication prompts
- Tailscale handles device identity via MagicDNS and issues TLS certificates automatically — the browser shows a valid HTTPS connection

Emails, notifications, texts:
- No email, push, or SMS notifications in v1
- App health state changes (an app goes unreachable or comes back online) are reflected in real time on the dashboard (status indicator on each app icon)
- Health events are logged by the controller and accessible via `oasis logs`
- Future consideration: optional webhook notifications for health state transitions (e.g. to a Slack channel or ntfy topic)
