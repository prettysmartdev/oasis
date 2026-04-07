# User Interface

## Style

Aesthetic:
- Clean, minimal phone/tablet homescreen aesthetic — a calm, organized homescreen amid the chaos of vibe-coding
	- The homescreen has two pages. The first page is labeled "AGENTS". The second page is labeled "APPS". Both titles are always visible at the top of the homescreen above the icons, AGENTS on the left and APPS on the right.
	- Each page vertically scrolls freely as more icons get added. If the user scrolls horizontally between the two pages, the scrolling snaps to each page (it's not freeform)
	- As the user moves horizontally between the two pages, the two titles change their size and opacity to indicate which page the user is on. The two titles should fluidly animate as the user moves the page left and right, with the "incoming" page's title becoming slightly larger and more opaque, while the "outgoing" page's title becomes smaller and more translucent.
- Background is a subtle color gradient generated dynamically based on the time of day (orange/red for sunrise/sunset, blue/white for midday, black/silver for night)
- Dark mode UI by default with light mode support; respects the user's system color-scheme preference
- Generous icons; 3x4 rounded-rect icon-based layout a la iOS where each app is a distinct, modern icon which can be an image or an emoji with a solid-colored background.
- Subtle, purposeful animations (icon hover lift, status indicator pulse, background slowly moves and morphs); nothing distracting

Brand and colors:
- Name: OaSis
- Primary: muted teal-green (#2DD4BF range) — evokes calm, nature, a place of refuge
- Accent: warm amber (#F59E0B range) — used for interactive CTAs, "Open" buttons, focus rings
- Neutrals: cool gray scale for borders, and secondary text (Tailwind slate palette)
- Font: Geist Sans for UI text, Geist Mono for code snippets and technical values (e.g. upstream URLs)
- Logo: stylized palm tree; simple enough to render at 16px favicon size

Desktop vs mobile:
- Mobile-first design; the primary use case is opening the homescreen on a phone or tablet to navigate to an app
- Fully responsive: icon grid collapses from 6 columns (1280px+) to 4 (1024px) to 3 (mobile)
- Touch targets sized for mobile use (minimum 44px); the homescreen should be optimized for a phone on the tailnet but usable on desktop

## Usage

Layout:
- Bottom navigation: 
	- OaSis logo (the palm tree emoji, within a circle) floating bottom left (not too big, modern floating, irridecent shimmering vibe, reacts to phone tilt to change shimmer interactively), 
	- Settings icon (gear emoji, within a circle) with "system status" pulsing status indicator attached bottom right. 
	- Tapping OaSis logo causes it to slide over to the right, with a messaging app-style chat box trailing it. Full-width at bottom of screen on mobile, stays docked bottom-left on tablet/desktop. Tapping chat box opens a chatbot-style conversation over top of the icon grid (shimmering background, chat bubbles).
- Main content area: responsive icon grid of registered agents/apps, optionally grouped by tags
- Each agent/app icon: app icon (emoji or image, rounded corners), display name (bold, below icon), health status indicator light (Healthy / Unreachable) to the right of the label.

Menus:
- Minimal navigation; almost all management is done via the CLI, not the dashboard
- Settings icon in the nav opens to a read-only settings card (hostname, version, registered app count, health of components) — configuration requires the CLI

Empty states:
- First launch (no agents/apps registered): 
	- agent page: centered illustration of a desert with a robot standing under a single palm tree, headline "Your OaSis awaits".
	- apps page centered illustration of a lake with a small log cabin, headline "Your OaSis awaits"
	- subline on both pages: "Add your first agent/app from the terminal", and a code block showing the `oasis app add` CLI command.
- App icon for an unreachable upstream: card is slightly desaturated, health indicator pulsing amber, tapping opens a card explaining the error (controller should provide basic details like HTTP error code or other.)

Accessibility:
- Semantic HTML throughout; proper heading hierarchy (h1 for page title, h2 for section headings, h3 for card names)
- All interactive elements are keyboard-navigable with visible focus rings (using the accent amber color)
- ARIA labels on icon-only buttons (settings icon, OaSis icon) and status badges
- Minimum WCAG AA contrast ratios for all text and interactive elements
- Respect prefers-reduced-motion: disable card hover animations and status pulse for users who have enabled reduced motion

Machine use:
- The controller's management API is the primary machine interface; use it for scripting, automation, and integrations
- The webapp does not expose a machine-readable interface; all programmatic access should use the management API
- The OaSis CLI wraps the management API for convenient scripting from the host machine

## Progressive Web App (PWA)

OaSis is a fully installable PWA. Any device on the tailnet can add the dashboard to its homescreen and launch it as a standalone app — no browser chrome, themed status bar, persistent icon.

### Web App Manifest

`webapp/public/manifest.json` declares the installability contract:

| Field | Value | Purpose |
|---|---|---|
| `name` | `OaSis` | Full name shown in install dialogs and app drawers |
| `short_name` | `OaSis` | Name shown under the homescreen icon on space-constrained launchers |
| `display` | `standalone` | Removes browser chrome (address bar, tabs) when launched from homescreen |
| `orientation` | `portrait-primary` | Locks portrait orientation; dashboard is not optimized for landscape |
| `background_color` | `#0f172a` (slate-900) | Shown while the app is loading; matches night-mode background to prevent white flash |
| `theme_color` | `#2DD4BF` (teal) | Android status bar and Chrome tab strip color; matches primary brand color |
| `start_url` | `/` | App launches at the root of the tsnet hostname |

The manifest is served at `/manifest.json` from the static export root. It must stay in sync with `aspec/uxui/interface.md` brand colors — if the palette changes, update both.

### Icons

Four PNG icons live at `webapp/public/icons/` and are committed as source assets:

| File | Size | Purpose |
|---|---|---|
| `icon-192.png` | 192×192 | Standard Android/Chrome homescreen icon |
| `icon-512.png` | 512×512 | Android splash screen and PWA install dialogs |
| `icon-maskable-192.png` | 192×192 | Android adaptive icon (safe zone: center 80%) |
| `icon-maskable-512.png` | 512×512 | Android adaptive icon, large variant |

`webapp/public/apple-touch-icon.png` at 180×180 is used by iOS Safari's "Add to Home Screen" flow (iOS ignores the manifest `icons` list).

The source SVG is at `webapp/public/icons/icon.svg` (palm tree / oasis motif). To regenerate all PNGs from the SVG:

```sh
make generate-icons
```

`make build-docker` runs `generate-icons` first automatically.

### Service Worker

The service worker (`sw.js`) is generated at build time by `@ducanh2912/next-pwa` (workbox) and placed at the export root so it is served from `/sw.js`. Its scope is `/`.

Caching strategy:

| Route pattern | Strategy | Notes |
|---|---|---|
| `/api/v1/*` | NetworkFirst | Always fetches fresh data; falls back to stale cache if offline (60 s max-age, 32 entries) |
| Static assets | CacheFirst | JS, CSS, images cached indefinitely; versioned filenames ensure cache busting |
| Navigation (HTML) | NetworkFirst → offline fallback | On failure, serves `/offline` |

**`sw.js` must not be cached by NGINX.** The NGINX config generator emits a `Cache-Control: no-store, no-cache, must-revalidate` header specifically for `location = /sw.js` so browsers always re-validate it on page load.

The service worker is disabled in `NODE_ENV=development` to avoid interfering with Next.js hot reload.

**Platform limitations:**
- iOS Safari does not support Background Sync or Push Notifications in PWAs. The offline fallback and manifest are the full extent of PWA support on iOS.
- iOS splash/launch images (`apple-touch-startup-image`) are out of scope; the `background_color` serves as a reasonable fallback.

### Offline Fallback

`/offline` is a static page (rendered from `webapp/app/offline/page.tsx`) served by the service worker when the network is unavailable and no cached document exists. It shows:

- A "You're offline" headline
- A reconnect prompt referencing the tailnet
- An animated wifi-off icon (animation disabled when `prefers-reduced-motion` is set)

### iOS Safe-Area Insets

`viewport-fit: cover` is set in the layout viewport config so app content extends behind the iOS notch and Dynamic Island. The `.bottom-nav-safe` CSS class applies `padding-bottom: env(safe-area-inset-bottom, 0px)` to the bottom navigation bar so buttons are not obscured by the iOS home indicator. This has no effect on Android or desktop.

### HTTPS Requirement

Service workers only register on HTTPS (or localhost). The oasis tailnet interface is served exclusively over HTTPS via tsnet's automatic TLS certificate (see work item 0002). PWA features will silently do nothing if HTTP is used — this invariant must remain intact.
