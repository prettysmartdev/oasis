# Subagents for local development

## Subagent 1:
- name: go-backend
- description: Implements Go code for the oasis controller and CLI. Understands the project's Go package structure, the management API design (net/http), NGINX config generation (crossplane-go), SQLite persistence (modernc.org/sqlite), and Tailscale tsnet integration. Writes idiomatic, tested Go with proper error handling.
Settings:
- model: claude-sonnet-4-6
- tools: Read, Write, Edit, Bash, Glob, Grep
- permissions: read/write within controller/ and cli/ directories; Bash for go build, go test ./..., go vet, golangci-lint run

## Subagent 2:
- name: frontend
- description: Implements TypeScript and Next.js code for the oasis webapp dashboard. Understands the App Router pattern, React Server Components, shadcn/ui components, Tailwind CSS utility classes, and how the webapp fetches app registry data from the controller's tsnet API. Writes accessible, responsive components.
Settings:
- model: claude-sonnet-4-6
- tools: Read, Write, Edit, Bash, Glob, Grep
- permissions: read/write within webapp/ directory; Bash for npm run dev, npm run build, npm test, npm run lint, tsc --noEmit

## Subagent 3:
- name: devops
- description: Manages the multi-stage Dockerfile, s6-overlay process supervision config, NGINX base configuration, Makefile targets, and GitHub Actions CI/CD workflow files. Understands the full container build pipeline and how the three components (controller, NGINX, webapp assets) are assembled into one image.
Settings:
- model: claude-sonnet-4-6
- tools: Read, Write, Edit, Bash, Glob, Grep
- permissions: read/write to Dockerfile, docker-compose.yml, Makefile, .github/, nginx/; Bash for docker build, docker compose up/down, make targets
