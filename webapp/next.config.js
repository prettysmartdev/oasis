const withPWA = require('@ducanh2912/next-pwa').default({
  dest: 'public', // sw.js and workbox-*.js land in webapp/public/ at build time
  cacheOnFrontEndNav: true,
  aggressiveFrontEndNavCaching: true,
  reloadOnOnline: true,
  disable: process.env.NODE_ENV === 'development',
  workboxOptions: {
    disableDevLogs: true,
    navigateFallback: '/offline',
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
  },
})

// NOTE: The webapp is served from the root of the tsnet hostname (e.g. https://oasis/).
// The service worker scope defaults to / and start_url in manifest.json is /.
// If NGINX ever mounts the webapp under a sub-path, both must be updated accordingly.
//
// NOTE: Service workers only register on HTTPS (or localhost). This is satisfied
// by tsnet's automatic TLS certificate provisioning (see work item 0002). If HTTP
// is used — e.g. a misconfigured reverse proxy — PWA features will silently do
// nothing (no install prompt, no offline fallback, no caching).
module.exports = withPWA({
  // Static export — no Next.js server runtime in the Docker container.
  // Served by NGINX from /srv/webapp.
  output: 'export',
  // Align with the Dockerfile COPY --from=webapp-builder /build/dist/webapp /srv/webapp
  distDir: '../dist/webapp',
})
