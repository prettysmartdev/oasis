/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export — no Next.js server runtime in the Docker container.
  // Served by NGINX from /srv/webapp.
  output: 'export',
  // Align with the Dockerfile COPY --from=webapp-builder /build/dist/webapp /srv/webapp
  distDir: '../dist/webapp',
}

module.exports = nextConfig
