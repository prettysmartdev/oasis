# Work Item: Feature

Title: Application proxying — accessType field and full-screen iFrame view
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary

Add an `accessType` field (`"direct"` | `"proxy"`) to the `App` data model. For `direct` apps the existing behaviour is preserved: tapping the icon opens the upstream URL in a new browser tab. For `proxy` apps the oasis controller configures NGINX to reverse-proxy all traffic hitting `/apps/<slug>/` to the upstream, and the oasis webapp opens the app in a full-screen iFrame (served through that NGINX route) instead of navigating away. A floating oasis icon remains above the iFrame at all times; tapping it reveals the chat bar and a home button that returns the user to the homescreen.

---

## User Stories

### User Story 1: Register a proxied app
As a: Owner / Admin

I want to:
run `oasis app add --name "My App" --slug my-app --upstream-url http://localhost:3000 --access-type proxy` (or include `accessType: proxy` in a YAML file) so the app is reverse-proxied through oasis at `/apps/my-app/`

So I can:
open the app inside the oasis homescreen as an iFrame without exposing its upstream port directly on the tailnet, keeping all traffic flowing through the single oasis NGINX gateway

### User Story 2: Open a proxied app from the homescreen
As a: Tailnet Visitor

I want to:
tap a proxy-type app icon and have it open inside a full-screen iFrame overlaid on the homescreen, with the oasis palm-tree icon always floating in the bottom-left corner

So I can:
use the app seamlessly within the oasis experience and return to the homescreen at any time without losing my place by tapping the oasis icon and then the home button

### User Story 3: Return to the homescreen from an open app
As a: Tailnet Visitor

I want to:
tap the oasis icon while a proxied app is open to reveal the chat bar (as normal) and a home button over top of the application that slides into the bottom-right corner

So I can:
close the iFrame and return to the homescreen icon grid with a single extra tap, without needing a back button or any browser navigation

---

## Implementation Details

### 1. Data model — `accessType` field

#### SQLite migration (migration 3)

Add `access_type` to the `apps` table via `ALTER TABLE`. The controller's `migrate` function in `internal/controller/db/store.go` must be updated to apply migration 3 when `PRAGMA user_version` is less than 3:

```sql
ALTER TABLE apps ADD COLUMN access_type TEXT NOT NULL DEFAULT 'direct';
PRAGMA user_version = 3;
```

Valid values: `"direct"` (default) | `"proxy"`. Existing rows default to `"direct"` with no data migration needed. Default for new applications added is `proxy` (and cli creating new YAML should default to proxy as well)

#### Go types (`internal/controller/db/store.go`)

Add `AccessType string` to the `App` struct (JSON: `"accessType"`). Add `AccessType *string` to `AppPatch`. Update `CreateApp`, `scanApp`, and `UpdateApp` to read and write the new column. `scanApp` must SELECT `access_type` in addition to all existing columns.

### 2. Management API — `accessType` in app endpoints

The `App` JSON object gains one field:

```
accessType: "direct" | "proxy"  (default "direct" on creation if omitted)
```

**Affected endpoints:**
- `GET /api/v1/apps` — include `accessType` in each item
- `POST /api/v1/apps` — accept optional `accessType`; default `"proxy"` when omitted; return 400 with `"code": "INVALID_ACCESS_TYPE"` for any other value
- `GET /api/v1/apps/:slug` — include `accessType`
- `PATCH /api/v1/apps/:slug` — allow updating `accessType`; same validation

Update the `App` definition in `aspec/architecture/apis.md` to add `accessType ("direct"|"proxy")` to the object description.

### 3. NGINX config generation (`internal/controller/nginx/config.go`)

Currently `buildConfig` generates a `location /apps/<slug>/` block for every enabled app. Change the filter to only emit proxy location blocks for enabled apps whose `AccessType == "proxy"`:

```go
for _, app := range apps {
    if !app.Enabled || app.AccessType != "proxy" {
        continue
    }
    // ... existing location block construction
}
```

Additionally, add standard proxy headers and strip `X-Frame-Options` so the iFrame can embed the upstream response:

```nginx
location /apps/<slug>/ {
    proxy_pass          <upstreamURL>/;
    proxy_set_header    Host              $host;
    proxy_set_header    X-Real-IP         $remote_addr;
    proxy_set_header    X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header    X-Forwarded-Proto $scheme;
    proxy_hide_header   X-Frame-Options;
    proxy_hide_header   Content-Security-Policy;
}
```

Use the crossplane AST to emit these as additional `Directive` entries inside the location block, consistent with the existing codestyle. A helper function `proxyHeaders() []crossplane.Directive` keeps the block construction tidy.

### 4. CLI changes (`internal/cli/app.go`)

#### `oasis app add`

Add `--access-type` flag (string, default `"proxy"`). Validate that the value is `"direct"` or `"proxy"` client-side before making the API call; exit 2 with `"--access-type must be one of: direct, proxy"` otherwise.

#### `oasis app update`

Add `--access-type` flag with the same validation as `app add`.
Ensure `oasis app update -f ./yaml-file.yaml` correctly reads, parses, validates, and patches an existing app (including the acces type field)

#### YAML template (`oasis app new`)

Add `accessType` field to the generated template (default 'proxy') and to `ParseAppFile`:

```yaml
accessType: "proxy"  # "direct" (open in new tab) | "proxy" (iFrame via oasis gateway)
```

`ParseAppFile` should validate the value is `"direct"` or `"proxy"` and include it in the error message for invalid values.

### 5. Webapp — state management

Lift the "active proxy app" state to the root `HomePage` component (`webapp/app/page.tsx`). Add:

```ts
const [activeProxyApp, setActiveProxyApp] = useState<App | null>(null)
```

Pass `onOpenProxyApp` down to `HomescreenLayout` (and into `AppIcon`) and `onCloseProxyApp` / `activeProxyApp` into `BottomNav`.

### 6. Webapp — `AppIcon` changes (`webapp/components/AppIcon.tsx`)

Add `accessType` to the `App` TypeScript interface in `webapp/lib/api.ts`:

```ts
accessType: 'direct' | 'proxy'
```

In `AppIcon`, when `app.accessType === 'proxy'` and `app.health === 'healthy'`, the click/keydown handler calls `onOpenProxyApp(app)` instead of following the `<a>` href. The unreachable-dialog path applies to proxy apps too (if health is `'unreachable'`, show the error dialog regardless of access type).

Update the component signature to accept an optional `onOpenProxyApp?: (app: App) => void` prop.

### 7. Webapp — `AppProxyView` component (`webapp/components/AppProxyView.tsx`)

New full-screen overlay component that renders the proxied app. Displayed only when `activeProxyApp !== null`.

```tsx
// Renders at z-30, beneath BottomNav (z-40) and above the homescreen grid.
<div className="fixed inset-0 z-30 bg-black">
  <iframe
    src={`/apps/${activeProxyApp.slug}/`}
    title={activeProxyApp.displayName}
    className="w-full h-full border-none"
    allow="fullscreen"
    // No sandbox attribute — the app is user-registered and served from the
    // same origin via NGINX proxy; sandbox would break most web apps.
  />
</div>
```

The component accepts `app: App` and is wrapped in `motion.div` for a fade-in transition (skip animation when `prefers-reduced-motion`). Export a named `AppProxyView`.

### 8. Webapp — `BottomNav` changes (`webapp/components/BottomNav.tsx`)

The settings icon should not be visible when a proxied app is open (proxied app iframe is ABOVE the settings button but BELOW the oasis button)

Add two props:

```ts
interface BottomNavProps {
  appOpen?: boolean      // true when a proxy app is currently displayed
  onCloseApp?: () => void // close the iFrame and return to homescreen
}
```

When `appOpen` is `true` and `chatOpen` is `true`, render a home-button that slides in from the right side (bottom-right corner):

```tsx
// Slides in when chatOpen && appOpen
<div
  className="overflow-hidden transition-all duration-300"
  style={{ width: (chatOpen && appOpen) ? '48px' : '0px', opacity: (chatOpen && appOpen) ? 1 : 0 }}
>
  <button
    onClick={onCloseApp}
    aria-label="Return to home screen"
    className="w-12 h-12 rounded-full bg-slate-800/90 border border-slate-700 flex items-center justify-center text-2xl shadow-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
  >
    🏠
  </button>
</div>
```

When `appOpen` is true, the settings button is hidden (it is irrelevant during full-screen app view). When `chatOpen` transitions from false → true while `appOpen` is true, the home button slides in via the same CSS transition used for the chat input. Respect `prefers-reduced-motion` by skipping the slide animation.

### 9. Webapp — `HomescreenLayout` changes

Pass `onOpenProxyApp` into `AppIcon` for each app:

```tsx
<AppIcon key={app.id} app={app} onOpenProxyApp={onOpenProxyApp} />
```

`HomescreenLayout` receives `onOpenProxyApp: (app: App) => void` as a prop from `HomePage`.

### 10. `aspec/architecture/apis.md` update

Add `accessType ("direct"|"proxy")` to the `App` object definition and add `INVALID_ACCESS_TYPE` to the validation rules section for `POST /api/v1/apps`.

---

## Edge Case Considerations

- **`accessType` omitted on creation** — default to `"proxy"`; never 400.
- **`accessType` with an invalid value** — return 400 with `{ "error": "...", "code": "INVALID_ACCESS_TYPE" }` from both the server (API handler) and the CLI (pre-flight validation, exit 2).
- **`direct` apps still have upstream URL** — `buildConfig` must no longer emit a proxy location block for `direct` apps. Confirm that removing the location block does not affect app health checks (health check logic in the controller probes `upstreamURL` directly, not via NGINX, so this is safe).
- **iFrame blocked by upstream `X-Frame-Options` or `Content-Security-Policy: frame-ancestors`** — The NGINX location block strips both `X-Frame-Options` and `Content-Security-Policy` response headers via `proxy_hide_header` so the browser will not refuse to embed the upstream response. Document this behaviour in a code comment; note that stripping CSP could weaken upstream app security — this is an explicit trade-off for the iFrame proxy experience.
- **Upstream app uses absolute paths (e.g. `/static/main.js`)** — NGINX `proxy_pass` with a trailing slash rewrites the path prefix correctly, but upstream apps that hard-code the root `/` path in asset references will break (assets will be fetched from `/static/main.js` instead of `/apps/<slug>/static/main.js`). This is a known limitation of path-prefix proxying. Document it as a known limitation in a code comment; do not attempt a general `sub_filter` fix in this work item.
- **Proxy app marked `enabled = false`** — `buildConfig` already skips disabled apps; the iFrame src `/apps/<slug>/` will return 404 from NGINX. `AppIcon` should prevent opening the iFrame for disabled apps (treat as `unknown` health for click purposes).
- **Proxy app health check returns `unreachable`** — Same dialog as `direct` apps: show the "App Unreachable" dialog. Do not open the iFrame.
- **User navigates within the iFrame to a page that sets its own `X-Frame-Options: DENY`** — The browser will blank the iFrame. No crash, but the user will see an empty frame. This is outside oasis's control; document as known limitation.
- **iFrame navigation back/forward** — The browser back button affects the outer window history, not the iFrame. This is expected; tapping the home button is the canonical way to leave the iFrame. Do not intercept browser history events.
- **Deep-linking into a proxy app** — There is no URL-level deep-link to an open proxy app in this work item (the homescreen is a single-page app with static export). This is acceptable for the initial implementation.
- **Mobile safe-area / notch** — The iFrame should cover the full viewport including safe-area. Use `inset: 0` (not `top: 0; left: 0; right: 0; bottom: 0`) and ensure the BottomNav retains the `bottom-nav-safe` class so the floating buttons remain above the notch.
- **Concurrent NGINX config regeneration** — `Configurator.Apply` already serialises writes with a mutex; no change needed. Ensure the new `AccessType` field is always available before `Apply` is called (it is, since it is read from DB before calling `Apply`).
- **Migration on existing database** — `ALTER TABLE apps ADD COLUMN` is safe on SQLite when the column has a `DEFAULT`. Verify the migration guard uses `version < 3` not `version == 2` so that future migrations can be appended.

---

## Test Considerations

### Unit tests

**`internal/controller/db`**
- `CreateApp` with `AccessType = "proxy"` persists the value and is retrievable by slug.
- `CreateApp` with `AccessType` omitted (zero value) stores `"direct"`.
- `UpdateApp` via `AppPatch.AccessType` changes the field and is reflected in the returned `App`.
- `ListApps` returns `AccessType` correctly for both `"direct"` and `"proxy"` apps.
- Migration 3 runs without error on a database at version 2 (simulate by opening an in-memory DB and running migrations 1 and 2 manually, then re-opening).

**`internal/controller/nginx`**
- `buildConfig` emits a `location /apps/<slug>/` block only for enabled `proxy` apps.
- `buildConfig` does NOT emit a location block for enabled `direct` apps.
- `buildConfig` emits `proxy_hide_header X-Frame-Options` and `proxy_hide_header Content-Security-Policy` in each proxy location block.
- `buildConfig` emits the four `proxy_set_header` directives in each proxy location block.
- An app that is disabled with `accessType = "proxy"` does not generate a location block.
- Existing tests that assert on config structure must be updated to match the new header directives.

**`internal/controller/api` (app handlers)**
- `POST /api/v1/apps` with `accessType: "proxy"` returns 201 with `accessType: "proxy"` in the response.
- `POST /api/v1/apps` with `accessType` omitted returns 201 with `accessType: "direct"` in the response.
- `POST /api/v1/apps` with `accessType: "invalid"` returns 400 with `INVALID_ACCESS_TYPE`.
- `PATCH /api/v1/apps/:slug` with `{ "accessType": "proxy" }` updates the field; subsequent GET returns `"proxy"`.
- `GET /api/v1/apps` and `GET /api/v1/apps/:slug` include `accessType` in the response body.

**`internal/cli` (app add/update)**
- `oasis app add --access-type proxy` calls the API with `accessType: "proxy"`.
- `oasis app add --access-type invalid` exits 2 with an error message before making any API call.
- `oasis app add` (no `--access-type` flag) sends `accessType: "direct"` (or omits it, relying on the server default).
- `oasis app update <slug> --access-type direct` sends a PATCH with `accessType: "direct"`.

**`internal/cli/yaml`**
- `ParseAppFile` with `accessType: proxy` parses correctly.
- `ParseAppFile` with `accessType: invalid` returns an error naming the field.
- `ParseAppFile` with `accessType` omitted defaults to `"direct"`.

### Integration tests (`//go:build integration`)

- Full lifecycle: `app add --access-type proxy` → `app list` shows the app with proxy access → verify NGINX config contains the location block with proxy headers → `app update --access-type direct` → verify NGINX config no longer contains the location block for that app.
- Full lifecycle: `app add --access-type direct` (or omitted) → verify NGINX config does not contain a location block for that app.

### Frontend tests

- `AppIcon` with `accessType: "proxy"` and `health: "healthy"` calls `onOpenProxyApp` and does not follow the `<a>` href.
- `AppIcon` with `accessType: "direct"` and `health: "healthy"` follows the link and does not call `onOpenProxyApp`.
- `AppIcon` with `accessType: "proxy"` and `health: "unreachable"` opens the error dialog, not the iFrame.
- `BottomNav` with `appOpen=true` and `chatOpen=true` renders the home button.
- `BottomNav` with `appOpen=false` does not render the home button even when `chatOpen=true`.
- `BottomNav` home button click calls `onCloseApp`.
- `AppProxyView` renders an `<iframe>` with `src="/apps/<slug>/"`.
- `AppProxyView` does not render when `activeProxyApp` is null.
- Reduced-motion: `AppProxyView` skips the fade-in animation when `prefers-reduced-motion` is set.

---

## Codebase Integration

- Follow established conventions, best practices, testing, and architecture patterns from the project's aspec.
- Management API changes must follow the REST conventions in `aspec/architecture/apis.md`; update the `App` object definition there as part of this work item.
- Controller changes must not break graceful NGINX reload or restart safety guarantees — `Configurator.Apply` uses SIGHUP and must remain unchanged.
- The `accessType` field must be validated server-side in the API handler (not just in the CLI) so that direct API callers get the same error response.
- All new and modified Go exported types and functions must carry godoc-style comments (critical invariant #9).
- The SQLite migration must use the existing `PRAGMA user_version` guard pattern in `internal/controller/db/store.go`. Bump `user_version` to 3.
- The `AppProxyView` component must respect `prefers-reduced-motion` for its entry animation.
- No new npm packages are needed for this work item; the iFrame requires no rendering library.
- Run `golangci-lint run ./internal/controller/... ./internal/cli/... ./cmd/...` before marking done; fix all reported issues.
- Run `npm run lint` and `tsc --noEmit` in `webapp/` before marking done.
- The `accessType` field in the YAML app template must be documented in `aspec/architecture/apis.md` alongside the existing YAML fields block.
