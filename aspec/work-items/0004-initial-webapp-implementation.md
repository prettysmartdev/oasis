# Work Item: Task

Title: Initial webapp implementation
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary:
- Implement the first functional version of the OaSis Next.js webapp dashboard: a responsive, iOS homescreen-style icon grid that displays registered agents and apps fetched from the controller's tsnet-facing API.
- Wire up the Next.js static export build so the compiled assets land in `dist/webapp/`, are copied correctly in the multi-stage Dockerfile, and are served by NGINX from `/srv/webapp`.
- This delivers the primary user-facing surface of OaSis — the dashboard any tailnet device can open to navigate to registered apps and agents.

## User Stories

### User Story 1:
As a: Tailnet Visitor

I want to:
open the OaSis dashboard in my browser from any device on my tailnet and see a responsive grid of all registered apps and agents with their health status

So I can:
quickly navigate to the tools I care about without remembering port numbers or URLs

### User Story 2 (if needed):
As a: Tailnet Visitor

I want to:
see meaningful empty-state screens on the Agents and Apps pages when nothing has been registered yet, with instructions for how to add something from the terminal

So I can:
understand what OaSis is and how to get started on first launch without needing separate documentation

### User Story 3 (if needed):
As a: Owner / Admin

I want to:
run `make docker-build` and get a single Docker image where NGINX serves the fully built Next.js static assets at the root path over the Tailscale interface

So I can:
ship a complete, working container image where the dashboard is immediately accessible after `oasis init` without any additional setup steps


## Implementation Details:

### Webapp structure (`webapp/`)

All new source lives under `webapp/`. The existing scaffold (`next.config.js`, `layout.tsx`, `globals.css`, `tailwind.config.js`) is retained and extended.

**New packages to install:**
- `geist` — already imported in layout.tsx via `next/font/google`; ensure it resolves correctly
- `@radix-ui/react-scroll-area`, `@radix-ui/react-dialog`, `@radix-ui/react-tooltip` — for scroll, settings card, icon tooltip
- `framer-motion` — purposeful animations (icon hover lift, background morph, page-title opacity/scale transitions); respect `prefers-reduced-motion`
- `clsx`, `tailwind-merge`, `class-variance-authority` — shadcn/ui utilities
- shadcn/ui components via CLI: `button`, `badge`, `dialog`, `tooltip`, `scroll-area`, `card`

**`webapp/app/page.tsx` — root page**
- Fetches `/api/v1/apps` from the controller's tsnet API at request time; because this is a static export (`output: 'export'`), data fetching must happen entirely client-side via `useEffect` / SWR / React Query — no `getServerSideProps` or React Server Component async data fetching that requires a server runtime.
- Use `NEXT_PUBLIC_API_BASE_URL` env var (default: empty string, i.e. same origin) so that local dev (`next dev`) can point at a running controller.
- Renders `<HomescreenLayout>` which hosts both pages.

**`webapp/components/HomescreenLayout.tsx`**
- Two-page horizontal scroll container (Agents | Apps) with snap-to-page CSS (`scroll-snap-type: x mandatory`; children `scroll-snap-align: start`).
- Tracks scroll position to animate the two page titles (AGENTS / APPS) — the active title renders at `text-xl font-bold opacity-100`, the inactive at `text-sm opacity-40`; animated via `framer-motion` `AnimatePresence` or a CSS transition driven by scroll ratio.
- On mobile: full-width paged scroll. On desktop: both columns visible side-by-side (≥1024px).
- Keyboard-navigable: left/right arrow keys switch pages.

**`webapp/components/AppIcon.tsx`**
- Renders a single app/agent icon: rounded-rect tile (iOS-style), icon (emoji or `<img>`) centered, display name below (bold, truncated), health indicator dot to the right of the label.
- Health indicator: green pulse for `healthy`, amber pulse for `unreachable`, gray static for `unknown`.
- Hover: subtle lift (`translateY(-2px) shadow-lg`) via framer-motion; disabled when `prefers-reduced-motion`.
- Tap/click: opens the upstream URL in a new tab if `healthy`; opens an error dialog if `unreachable`.
- ARIA: `role="link"` or `<a>` wrapping the tile; `aria-label="{displayName}, {health} status"`.

**`webapp/components/TimeOfDayBackground.tsx`**
- Generates a CSS gradient based on `new Date().getHours()` in the client:
  - 5–8 (sunrise): orange → red
  - 9–16 (midday): sky blue → white
  - 17–20 (sunset): orange → deep red
  - 21–4 (night): near-black → cool silver
- Rendered as a fixed full-viewport `<div>` behind the grid; slowly animates with a CSS keyframe (`background-position` shift over 60 s); disabled under `prefers-reduced-motion`.
- Re-evaluates on a 10-minute interval so long-lived sessions transition gracefully.

**`webapp/components/BottomNav.tsx`**
- OaSis logo (palm tree 🌴 emoji in an iridescent circle) floating bottom-left. Tapping slides it right and expands a chat input box (placeholder only in this work item — the chatbot feature is a future work item; the input should render but submit does nothing).
- Settings icon (⚙️ in a circle) floating bottom-right with a pulsing status indicator (green/amber/red) derived from the `/api/v1/status` response.
- Tapping the settings icon opens a `<Dialog>` (shadcn/ui) displaying: hostname, version, registered app count, Tailscale connection state, NGINX status — all fetched from `GET /api/v1/status`.
- No text labels on either nav buttons.

**`webapp/components/EmptyState.tsx`**
- Accepts `page: 'agents' | 'apps'` prop.
- Agents empty state: desert illustration (SVG), headline "Your OaSis awaits", subline "Add your first agent from the terminal", code block showing `oasis app add`.
- Apps empty state: lake + cabin illustration (SVG), headline "Your OaSis awaits", subline "Add your first app from the terminal", code block showing `oasis app add`.
- Uses `Geist Mono` (`font-mono`) for the code block.

**`webapp/lib/api.ts`**
- Typed fetch helpers for `GET /api/v1/apps` → `{ items: App[], total: number }` and `GET /api/v1/status` → `Status`.
- Types mirror the API object definitions in `aspec/architecture/apis.md`: `App`, `Status`, `AppHealth`.
- Reads `process.env.NEXT_PUBLIC_API_BASE_URL` for the base URL; defaults to `''` (same origin).
- Error handling: throws a typed `ApiError` with `status` and `message`; components render an error banner.

**`webapp/app/globals.css`**
- Add CSS custom properties for the time-of-day gradient and the iridescent logo shimmer animation.
- Dark mode: `:root { color-scheme: dark light }` respecting `prefers-color-scheme`.

**`webapp/tailwind.config.js`**
- Extend with the brand colors: `primary: '#2DD4BF'`, `accent: '#F59E0B'`, slate for neutrals.
- Font families: `sans: ['var(--font-geist-sans)', ...defaultTheme.fontFamily.sans]`, `mono: ['var(--font-geist-mono)', ...defaultTheme.fontFamily.mono]`.
- Safelist the health indicator pulse classes so they are not purged.

**`webapp/next.config.js`** — no changes required; `output: 'export'` and `distDir: '../dist/webapp'` are already correct.

### Build system

**Makefile** — ensure the `build` target runs `npm --prefix webapp ci && npm --prefix webapp run build` before building Go binaries. The `docker-build` target already calls the Dockerfile which handles this in-stage. Add a standalone `build-webapp` target:
```makefile
build-webapp:
	npm --prefix webapp ci
	npm --prefix webapp run build
```
Ensure `make build` depends on `build-webapp` so the full local build works without Docker.

**Dockerfile** — stage 1 (the `webapp-builder` stage) already runs `npm ci && npm run build` and the built output lands at `/build/dist/webapp` (relative to the repo root, i.e. `/build/dist/webapp` inside the builder). Verify the `COPY --from=webapp-builder` instruction copies to `/srv/webapp` in the final image. No changes expected — just confirm alignment.

**NGINX config** (generated by the controller via crossplane-go) — the controller's NGINX config generator must include a `root /srv/webapp` directive with `index index.html` and `try_files $uri $uri/ /index.html` for the dashboard location block. This is part of the controller's `internal/nginx` package; coordinate with the go-backend agent or leave a `// TODO` comment and a tracked work item if not already wired.

### Environment variables

- `NEXT_PUBLIC_API_BASE_URL` — added to `.env.local.example` with a comment: `# Base URL for the controller tsnet API (leave empty to use same origin in production)`
- In `docker-compose.dev.yml`, set `NEXT_PUBLIC_API_BASE_URL=http://localhost:04515` so the Next.js dev server can reach the local controller.

See `aspec/architecture/apis.md` for the full API object shapes and endpoint list.


## Edge Case Considerations:

- **Static export + client-side data fetching**: `output: 'export'` forbids async Server Components that fetch data and server-side rendering. All `/api/v1/*` calls must be made client-side (React `useEffect` or SWR). Ensure no `async` page components or Route Handlers are introduced — they will silently break the build or be excluded from the export.
- **Controller not yet running (local dev)**: when `NEXT_PUBLIC_API_BASE_URL` points to a controller that is not running, `fetch` will throw a network error. The webapp must catch this and display an error banner ("Cannot reach OaSis controller") rather than crashing or showing a blank screen.
- **Empty registry**: both the Agents and Apps pages must render their empty-state UI when `items` is `[]`; never show a blank white screen.
- **Mixed icon types**: the `icon` field is a string that can be an emoji (e.g. `"🤖"`) or an HTTPS image URL. `AppIcon` must detect which and render `<span>` (emoji) or `<img>` (URL) accordingly; broken image URLs should fall back to a default emoji (`"📦"`).
- **Long display names**: display names must be truncated with `text-ellipsis overflow-hidden whitespace-nowrap` to prevent layout breakage at narrow widths.
- **Rapid health state changes**: the status indicator should debounce or throttle re-renders; polling every 30 s is sufficient — do not hammer the API.
- **Background gradient on SSR/hydration**: `new Date().getHours()` differs between server render time and client hydration time (timezone, clock skew). Since the export is fully static, the gradient component must be client-only (`'use client'` + mounted state guard) to avoid hydration mismatch warnings.
- **`prefers-reduced-motion`**: all framer-motion animations and CSS keyframes must be gated on `useReducedMotion()` (framer-motion hook) and the `prefers-reduced-motion` media query in CSS. No animations should fire for users who have opted out.
- **Font loading in static export**: `next/font/google` inlines font CSS at build time. Verify fonts are embedded correctly in the export output and do not make runtime Google Fonts requests (which may be blocked on some tailnets or for offline use).
- **NGINX `try_files` for SPA routing**: the static export produces an `index.html` at the root; any sub-path navigations must fall through to `index.html`. The NGINX config must include `try_files $uri $uri/ /index.html =404` — if this is missing, direct URL access to any non-root path will 404.
- **`distDir` path mismatch**: `next.config.js` sets `distDir: '../dist/webapp'` (relative to `webapp/`), so the output lands at `dist/webapp/` in the repo root. The Dockerfile's `COPY --from=webapp-builder` path must exactly match. Any change to `distDir` must be reflected in the Dockerfile simultaneously.


## Test Considerations:

**Unit tests (Jest + React Testing Library, `webapp/__tests__/`):**
- `AppIcon.test.tsx`: renders with emoji icon, renders with image URL, shows health indicator for each health state (`healthy`/`unreachable`/`unknown`), truncates a long display name, renders an error dialog on click when `unreachable`.
- `EmptyState.test.tsx`: renders agents empty state with correct headline; renders apps empty state with correct headline; code block contains `oasis app add`.
- `TimeOfDayBackground.test.tsx`: given a mocked hour, applies the expected gradient CSS class or style; does not animate when `prefers-reduced-motion` is set (mock `window.matchMedia`).
- `HomescreenLayout.test.tsx`: renders both AGENTS and APPS titles; contains two page containers; keyboard arrow-key navigation emits a scroll or active-page state change.
- `BottomNav.test.tsx`: settings icon has ARIA label; tapping settings icon opens the settings dialog; logo button has ARIA label.
- `api.test.ts`: `fetchApps` returns typed `App[]` on 200; throws `ApiError` on non-200; handles network failure gracefully.
- Smoke test (`page.test.tsx`): existing smoke test must remain green; extend it to confirm the page renders without crashing when the API returns an empty list (mock `fetch`).

**Build verification:**
- CI `build` job must confirm `npm --prefix webapp run build` exits 0 and that `dist/webapp/index.html` exists.
- Docker build must succeed end-to-end (`make docker-build`); spot-check that `/srv/webapp/index.html` is present in the final image (`docker run --rm oasis ls /srv/webapp`).

**No integration tests in this work item** for the webapp specifically — NGINX serving the static assets is validated as part of the Docker build smoke check above. Full end-to-end browser tests are a future work item.

**Coverage threshold:** raise the `webapp` Jest coverage threshold from the current stub level to at least 50% line coverage for new files introduced in this work item. Update `jest.config.js` `coverageThreshold` accordingly.


## Codebase Integration:

- Follow established conventions, best practices, testing, and architecture patterns from the project's aspec.
- The `output: 'export'` constraint in `next.config.js` is a hard invariant (see `aspec/architecture/design.md`); never introduce dynamic routes, Route Handlers, or Server Actions.
- All API fetch calls use `NEXT_PUBLIC_API_BASE_URL` from the environment so the same build artifact works in Docker (same-origin NGINX proxy) and in local dev (pointing at the controller directly). See `aspec/devops/localdev.md` and `.env.local.example`.
- Brand colors and fonts must match `aspec/uxui/interface.md` exactly: primary `#2DD4BF`, accent `#F59E0B`, Geist Sans / Geist Mono.
- Accessibility requirements are non-negotiable: WCAG AA contrast, keyboard nav, ARIA labels, `prefers-reduced-motion`. See `aspec/uxui/interface.md`.
- The responsive grid column counts (6 / 4 / 3) must match the spec in `aspec/uxui/interface.md`.
- The `distDir` path in `next.config.js` and the `COPY --from=webapp-builder` path in the Dockerfile are a coupled invariant — changing one requires changing the other. Coordinate with the devops agent if a change is needed.
- NGINX `root /srv/webapp` and `try_files` configuration is generated by the controller's `internal/nginx` package (crossplane-go). If that package does not yet emit a dashboard location block, file a follow-up work item or coordinate with the go-backend agent to add it; do not hard-code NGINX config outside of the generator.
- New environment variables (`NEXT_PUBLIC_API_BASE_URL`) must be added to `.env.local.example` with a descriptive comment.
- The `webapp/components/` directory uses shadcn/ui conventions; install components via the shadcn CLI (`npx shadcn-ui@latest add <component>`) rather than copying manually, to keep upgrade paths clean.
