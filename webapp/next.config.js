/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export — no Next.js server runtime in the Docker container.
  // Served by NGINX from /srv/webapp.
  output: 'export',
  // Align with the Dockerfile COPY --from=webapp-builder /build/dist/webapp /srv/webapp
  distDir: '../dist/webapp',
  // TODO(nginx): ensure internal/nginx emits `try_files $uri $uri/ /index.html =404`
  // for the dashboard location block so that direct URL access to any sub-path does not 404.
}

module.exports = nextConfig
