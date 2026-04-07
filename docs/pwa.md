# Installing OaSis as an App

OaSis is a Progressive Web App (PWA). Any device on your tailnet can install the dashboard to its homescreen and launch it as a native-feeling standalone app — no browser chrome, no address bar, and a dedicated icon that looks and behaves like any other app on your phone.

## iOS Safari

1. Open your OaSis URL (`https://oasis.[tailnet-name].ts.net`) in **Safari** on your iPhone or iPad.
   - PWA installation does not work from Chrome, Firefox, or other third-party browsers on iOS.
2. Tap the **Share** button (the box with an arrow pointing upward, in the toolbar).
3. Scroll down and tap **Add to Home Screen**.
4. Optionally edit the name — it defaults to **OaSis**.
5. Tap **Add** in the top-right corner.

The OaSis icon now appears on your homescreen. Tap it to launch the dashboard in full-screen standalone mode — no Safari browser chrome.

**Offline behavior:** If your tailnet connection drops while using the installed app, OaSis shows a friendly offline screen instead of a browser error page. Reconnect to your tailnet and the app resumes normally.

**Note:** iOS Safari does not support web push notifications or background sync in PWAs. OaSis does not require either of these.

## Android (Chrome)

1. Open your OaSis URL in **Chrome** on your Android device.
2. Chrome will display an **"Add OaSis to Home screen"** banner at the bottom of the screen after a moment, or you can trigger it manually:
   - Tap the **⋮ menu** (three dots, top-right).
   - Tap **Add to Home screen**.
3. Tap **Add** in the confirmation dialog.
4. Optionally drag the icon to your preferred position on the homescreen.

The OaSis icon appears on your homescreen and in your app drawer. Tap it to launch OaSis in standalone mode with the teal status bar.

**Offline behavior:** Same as iOS — OaSis shows the offline fallback page when the tailnet is unreachable.

## Desktop (Chrome / Edge)

1. Open your OaSis URL in Chrome or Edge.
2. Look for the **install icon** (a monitor with a downward arrow) in the browser address bar, or:
   - Chrome: click ⋮ → **Cast, save, and share** → **Install page as app**.
   - Edge: click **…** → **Apps** → **Install this site as an app**.
3. Confirm in the dialog.

OaSis opens in its own window without the browser toolbar.

## Verifying the Installation

After installing, you can confirm the PWA is working correctly:

- **Standalone mode:** Launch from the homescreen. There should be no browser address bar or tab strip — only the OaSis dashboard.
- **Theme color:** On Android, the status bar at the top of the screen turns teal (`#2DD4BF`). On iOS, the status bar text becomes white over a translucent background.
- **Icon:** The OaSis palm tree icon should appear with correct colors and no unexpected clipping on Android (the maskable icon variant handles the adaptive icon safe zone).
- **Offline fallback:** Enable Airplane Mode (or disconnect from your tailnet), then open the installed app. The "You're offline" page should appear within a few seconds instead of a browser network error.

## Troubleshooting

**The install prompt doesn't appear on Android.**
Chrome requires the page to be served over HTTPS and have a valid service worker registered. OaSis satisfies both via tsnet's automatic TLS and the workbox service worker built into the static export. If the prompt still doesn't appear, make sure:
- You are accessing the tailnet URL (not `localhost`).
- The Tailscale connection is active and the `oasis` node is reachable.
- Chrome has not previously dismissed the install prompt for this origin (clear site data and try again).

**The icon looks wrong (clipped or stretched) on Android.**
Android adaptive icons use the maskable variants (`icon-maskable-192.png`, `icon-maskable-512.png`). The icon content is designed to fit within the central 80% safe zone. If you have customized `icon.svg`, run `make generate-icons` and redeploy; Android may cache the old icon for a few minutes after a reinstall.

**The offline page doesn't appear; I see a browser error instead.**
The service worker must be registered and have cached at least one navigation response before going offline for the fallback to work. Visit the dashboard at least once while online after installation, then test the offline scenario.

**PWA features don't work at all on iOS.**
Ensure you are using Safari, not a third-party browser. iOS restricts PWA installation and service worker registration to Safari only.

## For Developers

The PWA implementation consists of:

| File / path | Purpose |
|---|---|
| `webapp/public/manifest.json` | Web App Manifest (name, icons, display mode, colors) |
| `webapp/public/icons/` | PNG icons committed as source assets |
| `webapp/public/icons/icon.svg` | Source SVG — edit this, then run `make generate-icons` |
| `webapp/public/apple-touch-icon.png` | 180×180 icon for iOS Safari "Add to Home Screen" |
| `webapp/app/offline/page.tsx` | Static offline fallback page |
| `webapp/next.config.js` | `@ducanh2912/next-pwa` wrapper with workbox config |
| `webapp/app/layout.tsx` | `metadata` (manifest, appleWebApp, icons) and `viewport` (themeColor, viewportFit) exports |
| `webapp/app/globals.css` | `.bottom-nav-safe` utility for iOS home indicator padding |
| `internal/controller/nginx/config.go` | Emits `Cache-Control: no-store` for `/sw.js` location |

The service worker (`sw.js`) and workbox runtime (`workbox-*.js`) are generated at build time by `npm run build` and placed in `webapp/public/`. They are **not** committed to the repository (`.gitignore` excludes them) and are produced fresh in every Docker image build.

See `aspec/uxui/interface.md` for the full PWA design specification.
