# Work Item: Enhancement

Title: Progressive webapp (PWA) — installable on iOS and Android
Issue: <GitHub issue URL, e.g. https://github.com/[owner]/oasis/issues/42>

## Summary:
- Add full Progressive Web App support to the oasis Next.js static export so that any tailnet user can install the dashboard to their iOS or Android homescreen and use it with a native-app feel (standalone full-screen mode, themed status bar, app icon, and offline fallback).
- This requires a Web App Manifest, a service worker for asset caching and offline support, iOS-specific meta tags, and correctly-sized maskable icons — all compatible with the existing `output: 'export'` static build constraint.
- Installing the dashboard removes the browser chrome, makes the app instantly launchable from the homescreen, and reinforces the "personal homescreen" identity of oasis across every device on the tailnet.


## User Stories

### User Story 1:
As a: Tailnet Visitor

I want to:
tap "Add to Home Screen" on iOS Safari (or use the Chrome install prompt on Android) and have the oasis dashboard install as a standalone app with the correct icon, name, and theme color

So I can:
launch oasis from my homescreen like a native app — full screen, no browser chrome — the same way I would open any other app on my phone

### User Story 2:
As a: Tailnet Visitor

I want to:
open the installed oasis app when my device is temporarily offline or my tailnet connection drops and see a friendly offline fallback screen instead of a browser error page

So I can:
understand that the connection is lost without being confused by a raw browser network error, and know the app itself is healthy

### User Story 3:
As a: Owner / Admin

I want to:
run `make build` or `make docker-build` and get a Docker image where the oasis webapp is automatically PWA-compliant — manifest, service worker, and icons all wired up — with no extra configuration steps

So I can:
ship the PWA capability as a built-in feature of every oasis release without any per-device or per-user setup


## Implementation Details:

### Package: `next-pwa`

Install `next-pwa` (Ducanh's fork: `@ducanh2912/next-pwa`) which is maintained and supports `output: 'export'` with workbox. This generates a service worker (`sw.js`) at build time that is placed at the root of the static export output.

```
npm --prefix webapp install @ducanh2912/next-pwa workbox-window
npm --prefix webapp install --save-dev @types/workbox-window
```

`next-pwa` wraps the Next.js config; update `webapp/next.config.js`:

```js
const withPWA = require('@ducanh2912/next-pwa').default({
  dest: 'public',           // sw.js and workbox-*.js land in webapp/public/ at build time
  cacheOnFrontEndNav: true,
  aggressiveFrontEndNavCaching: true,
  reloadOnOnline: true,
  disable: process.env.NODE_ENV === 'development',
  workboxOptions: {
    disableDevLogs: true,
  },
})

module.exports = withPWA({
  output: 'export',
  distDir: '../dist/webapp',
})
```

Because `output: 'export'` copies everything in `public/` into the export root, the generated `sw.js` and `workbox-*.js` files will be present at the root of `dist/webapp/` and therefore served by NGINX at `https://<oasis-host>/sw.js`. This is the required registration path for a service worker.

### Web App Manifest (`webapp/public/manifest.json`)

Create `webapp/public/manifest.json`:

```json
{
  "name": "OaSis",
  "short_name": "OaSis",
  "description": "Your homescreen for vibe-coded apps and agents",
  "start_url": "/",
  "display": "standalone",
  "orientation": "portrait-primary",
  "background_color": "#0f172a",
  "theme_color": "#2DD4BF",
  "icons": [
    { "src": "/icons/icon-192.png",   "sizes": "192x192",   "type": "image/png" },
    { "src": "/icons/icon-512.png",   "sizes": "512x512",   "type": "image/png" },
    { "src": "/icons/icon-maskable-192.png", "sizes": "192x192", "type": "image/png", "purpose": "maskable" },
    { "src": "/icons/icon-maskable-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable" }
  ]
}
```

- `display: "standalone"` removes Safari/Chrome browser chrome when launched from homescreen.
- `background_color` matches the night-mode background (`#0f172a`, Tailwind `slate-900`) to prevent a white flash on launch.
- `theme_color` matches the primary brand color (`#2DD4BF`) so the Android status bar and Chrome tab strip adopt the teal tone.
- `start_url: "/"` must be a relative path — the app is served from the root of the tsnet hostname, so no path prefix is needed.

### App Icons (`webapp/public/icons/`)

Generate four PNG icons using a palm tree / oasis motif consistent with the existing `🌴` brand identity:

- `icon-192.png` — standard 192×192 for Android/Chrome
- `icon-512.png` — standard 512×512 for Android splash and PWA install dialogs
- `icon-maskable-192.png` — 192×192 with `purpose: maskable`; the icon content must fit within the safe zone (center 80% circle) so Android adaptive icons do not clip the design
- `icon-maskable-512.png` — 512×512 maskable variant

Commit the source SVG at `webapp/public/icons/icon.svg` so icons can be regenerated. Add a `Makefile` target `make generate-icons` that uses `sharp-cli` or `squoosh-cli` (installed as a dev dependency) to produce all four PNGs from the SVG. Include this as a dependent target to `make docker-build`.

Additionally add `webapp/public/apple-touch-icon.png` at 180×180 for iOS Safari's "Add to Home Screen" icon (iOS ignores the manifest icons list and looks for this link tag specifically). Ensure the script/make target handle this as well.

### `<head>` meta tags (`webapp/app/layout.tsx`)

Extend the `metadata` export and add an explicit `<link>` for the Apple touch icon. Replace the existing `metadata` object:

```tsx
export const metadata: Metadata = {
  title: 'OaSis',
  description: 'Your homescreen for vibe-coded apps and agents',
  manifest: '/manifest.json',
  themeColor: '#2DD4BF',
  appleWebApp: {
    capable: true,
    statusBarStyle: 'black-translucent',
    title: 'OaSis',
  },
  viewport: {
    width: 'device-width',
    initialScale: 1,
    maximumScale: 1,
    userScalable: false,
    viewportFit: 'cover',
  },
  icons: {
    apple: '/apple-touch-icon.png',
  },
}
```

`Next.js` Metadata API translates the above to the correct `<meta>` and `<link>` tags including:
- `<link rel="manifest" href="/manifest.json">`
- `<meta name="theme-color" content="#2DD4BF">`
- `<meta name="apple-mobile-web-app-capable" content="yes">`
- `<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">`
- `<meta name="apple-mobile-web-app-title" content="OaSis">`
- `<link rel="apple-touch-icon" href="/apple-touch-icon.png">`

`viewport-fit: cover` is required so the app content extends behind the iOS notch / dynamic island; pair this with CSS `env(safe-area-inset-*)` padding in `globals.css` for the bottom nav area (see below).

### Safe-area insets (`webapp/app/globals.css`)

Add padding rules for the bottom navigation area so content is not hidden behind the iOS home indicator:

```css
.bottom-nav-safe {
  padding-bottom: env(safe-area-inset-bottom, 0px);
}
```

Apply `.bottom-nav-safe` to `BottomNav.tsx`'s outermost container. This ensures the floating bottom buttons sit above the iOS home indicator bar in standalone mode.

### Offline fallback page (`webapp/app/offline/page.tsx`)

Create a client-only page at `webapp/app/offline/page.tsx` that renders a friendly offline message:
- Headline: "You're offline"
- Subline: "OaSis couldn't reach your tailnet. Reconnect and try again."
- Animated wifi-off SVG icon (respect `prefers-reduced-motion`)
- No data fetching — pure static markup

Register the offline fallback in `next.config.js` workbox options:

```js
workboxOptions: {
  disableDevLogs: true,
  offlineFallbacks: {
    document: '/offline',
  },
},
```

Because `output: 'export'` generates `/offline/index.html`, Next.js static export will produce this page at the correct path.

### Service worker registration

`next-pwa` automatically injects a `<script>` that registers `sw.js` via `workbox-window`. No manual registration code is required. The service worker uses a cache-first strategy for static assets and a network-first strategy for `/api/v1/*` routes (so that API responses are always fresh when online, with a stale-while-revalidate fallback).

Optionally add custom runtime caching in `next.config.js` workbox options:

```js
runtimeCaching: [
  {
    urlPattern: /^\/api\/v1\/.*/i,
    handler: 'NetworkFirst',
    options: {
      cacheName: 'api-cache',
      networkTimeoutSeconds: 10,
      expiration: { maxEntries: 32, maxAgeSeconds: 60 },
    },
  },
],
```

### `webapp/package.json` changes

- Add `@ducanh2912/next-pwa` and `workbox-window` to `dependencies`.
- The generated `sw.js` and `workbox-*.js` files land in `webapp/public/` at build time. Add them to `.gitignore` (`webapp/public/sw.js`, `webapp/public/workbox-*.js`) so they are not committed; they are produced fresh by each build.

### Dockerfile / build system

No Dockerfile changes are required. The `webapp-builder` stage runs `npm ci && npm run build`, which now also runs `next-pwa`'s workbox generation. The output in `dist/webapp/` will include `sw.js` at the root, which NGINX serves at `/sw.js`. Verify the NGINX location block does not block `.js` files at the root path.

### NGINX cache headers

The service worker file must not be cached by NGINX itself (or by intermediary caches) because browsers re-validate it on every load to pick up updates. Add a `Cache-Control: no-store` header for `/sw.js` in the controller's NGINX config generator (`internal/nginx`):

```nginx
location = /sw.js {
    root /srv/webapp;
    add_header Cache-Control "no-store, no-cache, must-revalidate";
}
```

Coordinate with the go-backend agent to add this location block via crossplane-go. If the nginx generator does not yet support per-location headers, add a `// TODO` and file a follow-up work item.


## Edge Case Considerations:

- **`output: 'export'` + service worker scope**: the service worker is served from `/sw.js` at the root of the tsnet hostname. Its scope defaults to `/`. This is correct as long as the webapp is served from the root path; if NGINX ever mounts it under a sub-path (e.g. `/dashboard/`), the scope and `start_url` in the manifest would both need updating. Document this assumption in a comment in `next.config.js`.
- **iOS Safari PWA limitations**: iOS does not support the Background Sync or Push Notification APIs for PWAs. Do not design any feature in this work item that relies on these. The offline fallback and manifest are the full extent of PWA support on iOS for now.
- **iOS splash screens (launch images)**: iOS Safari can display a custom launch image while the app loads if `apple-touch-startup-image` link tags are present. These require a separate image per device resolution (there are ~10+ sizes). This is out of scope for the initial implementation; add a follow-up note in the work item. The `background_color` in the manifest serves as a reasonable fallback.
- **Service worker update UX**: when a new version of oasis is deployed, the service worker will update in the background. The `reloadOnOnline: true` option in `next-pwa` handles re-activation; no explicit "new version available — reload" prompt is needed for the initial implementation. Add a follow-up work item if more control is desired.
- **HTTPS requirement**: service workers only register on HTTPS (or localhost). The oasis tailnet interface is already served over HTTPS via tsnet's automatic TLS (see work item 0002). This invariant must remain intact; PWA features will silently do nothing if HTTP is used. No code change needed, but document the dependency.
- **`sw.js` in `.gitignore` but needed in Docker build**: the generated `sw.js` is in `webapp/public/` which is in the Docker build context. Because the Dockerfile runs `npm ci && npm run build` inside the container, the file is generated fresh in the image and never needs to be committed. Confirm the `.gitignore` entry is scoped to `webapp/public/sw.js` (not a broad `/public/*.js` that could exclude other intentional public JS files).
- **`workbox-*.js` filename includes a hash**: workbox generates a content-hashed filename (e.g. `workbox-abc123.js`) on each build. Gitignore the pattern `webapp/public/workbox-*.js` to cover all variants.
- **Maskable icon safe zone**: if the icon art extends beyond the central 80% circle of the maskable variants, Android adaptive icons will clip it. Validate maskable icons with maskable.app before committing. The standard (non-maskable) icons should fill the full canvas without safe-zone padding.
- **`viewport-fit: cover` and bottom nav on Android**: `env(safe-area-inset-bottom)` resolves to `0px` on most Android devices (gesture bar is handled differently). The `.bottom-nav-safe` padding is harmless but effectively a no-op there — no need for platform detection.
- **`maximumScale: 1` / `userScalable: false`**: disabling pinch-zoom is an accessibility concern. It is acceptable here because the oasis dashboard is an icon launcher (not a reading surface), but document the trade-off. If a user story requiring text accessibility arises, this should be revisited.
- **Dev mode**: `next-pwa` is disabled in `NODE_ENV=development` (configured explicitly) to prevent service worker interference with hot reload. Ensure `jest` tests do not accidentally run in a mode where the PWA wrapper executes.
- **Manifest `start_url` vs. actual URL**: the manifest specifies `start_url: "/"`. If the tailnet hostname changes (e.g. the user renames the oasis node), the installed PWA's `start_url` will resolve relative to the current hostname, so there is no stale-URL problem. No special handling needed.


## Test Considerations:

**Unit tests (Jest + React Testing Library, `webapp/__tests__/`):**
- `offline.test.tsx`: renders the offline page without crashing; headline "You're offline" is present; no network calls are made (assert `fetch` is never called).
- `layout.test.tsx` (or extend existing smoke test): assert that the rendered `<head>` contains `<link rel="manifest">`, `<meta name="theme-color">`, `<meta name="apple-mobile-web-app-capable">`, and `<link rel="apple-touch-icon">`. Use `@testing-library/jest-dom` to inspect the document head after rendering `RootLayout`.
- `BottomNav.test.tsx` (extend existing): assert the outermost container has the `bottom-nav-safe` class so safe-area padding is applied.

**Build verification (CI):**
- After `npm --prefix webapp run build`, assert that `dist/webapp/sw.js` exists (`test -f dist/webapp/sw.js`).
- Assert that `dist/webapp/manifest.json` exists and is valid JSON with required fields (`name`, `short_name`, `icons`, `start_url`, `display`).
- Assert that `dist/webapp/icons/icon-192.png`, `icon-512.png`, `icon-maskable-192.png`, `icon-maskable-512.png`, and `apple-touch-icon.png` all exist.
- Add these assertions to the CI `build` job in `.github/workflows/ci.yml` as shell steps after the `npm run build` step.

**Manual / device testing (not automated, but required before closing the work item):**
- iOS Safari: open the dashboard, tap Share → Add to Home Screen; confirm icon, name, and standalone launch (no browser chrome).
- iOS Safari (offline): disconnect from tailnet, launch from homescreen; confirm offline fallback page renders instead of a browser error.
- Android Chrome: open the dashboard; confirm the install banner / "Add to Home Screen" prompt appears; install and launch; confirm standalone mode and teal status bar.
- Android Chrome (offline): same offline test as iOS.
- Run the manifest through [web.dev/measure](https://web.dev/measure) or Lighthouse PWA audit (targeting the tsnet URL) and confirm "Installable" and "PWA Optimized" categories pass. This can be done manually by the developer; it is not automated in CI.

**No integration tests** are added in this work item for the service worker itself — workbox unit testing requires a browser environment (jsdom does not support service workers). The build verification steps above are the automated gate.


## Codebase Integration:

- The `output: 'export'` hard constraint in `next.config.js` is unchanged; `next-pwa` with `@ducanh2912/next-pwa` is the only PWA library confirmed to work correctly with Next.js static export. Do not use the older `next-pwa` (without the `@ducanh2912` scope) — it is unmaintained and has known issues with Next.js 14+.
- The `distDir: '../dist/webapp'` path is unchanged. The workbox `dest: 'public'` places generated files in `webapp/public/`, which Next.js static export copies to the output root. Confirm these two paths are aligned after any future `distDir` change.
- Brand colors in `manifest.json` (`theme_color: "#2DD4BF"`, `background_color: "#0f172a"`) must stay in sync with `aspec/uxui/interface.md`. If the brand palette changes, update both.
- The NGINX `Cache-Control: no-store` header for `/sw.js` must be emitted by the controller's crossplane-go NGINX config generator (`internal/controller/nginx`), not hard-coded in a static NGINX config file outside the generator. Follow the patterns established in work item 0002. Coordinate with the go-backend agent.
- The `apple-touch-icon.png` at 180×180 and the four icon PNGs under `webapp/public/icons/` must be committed to the repository (they are source assets, not build artifacts). Only the generated `sw.js` and `workbox-*.js` are gitignored.
- The generated `sw.js` and `workbox-*.js` must be added to `webapp/public/.gitignore` (scope-limited so other intentional public assets are not affected).
- The `Metadata` API changes in `layout.tsx` use the Next.js 14 `metadata` export pattern (not `<Head>` from `next/head`). This is consistent with App Router conventions already in use. Do not mix the two approaches.
- `viewport-fit: cover` and `env(safe-area-inset-*)` are only meaningful in standalone PWA mode on iOS. They have no negative effect in a normal browser session. The CSS change in `globals.css` is safe to apply unconditionally.
- Follow established conventions, best practices, testing, and architecture patterns from the project's aspec.
