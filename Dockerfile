# syntax=docker/dockerfile:1

# ── Stage 1: Build Next.js webapp ──────────────────────────────────────────────
FROM node:20-alpine AS webapp-builder
WORKDIR /build

# Install dependencies first (layer-cached unless package.json changes)
COPY webapp/package*.json ./webapp/
RUN --mount=type=cache,target=/root/.npm \
    npm --prefix webapp ci

# Copy source and build static export
COPY webapp/ ./webapp/
RUN npm --prefix webapp run build
# Output lands at /build/dist/webapp (next.config.js: distDir: '../dist/webapp')


# ── Stage 2: Build Go binaries ──────────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder
WORKDIR /build

# Download dependencies (cached in /go/pkg/mod across builds).
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy only Go source — changes to webapp/, docs/, aspec/, etc. won't bust this layer.
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build with CGO_ENABLED=0 for fully static binaries.
# --mount=type=cache,target=/root/.cache/go-build persists the Go compiler's
# incremental build cache across Docker builds, turning a 200 s cold compile
# into a few seconds when only application code has changed.
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
        -mod=mod \
        -ldflags "-X main.version=${VERSION}" \
        -o bin/controller ./cmd/controller \
    && CGO_ENABLED=0 GOOS=linux go build \
        -mod=mod \
        -ldflags "-X main.version=${VERSION}" \
        -o bin/oasis ./cmd/oasis


# ── Stage 3: Runtime image ─────────────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

# Pin s6-overlay version to avoid unexpected breakage on image rebuilds.
ARG S6_OVERLAY_VERSION=3.1.6.2

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        nginx \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Install s6-overlay for process supervision (controller + NGINX).
# TARGETARCH is set automatically by Docker BuildKit for multi-platform builds
# (linux/amd64 → x86_64, linux/arm64 → aarch64). The ADD instruction does not
# support ARG substitution in URLs, so we use curl with a shell conditional instead.
ARG TARGETARCH=amd64
RUN S6_ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "aarch64" || echo "x86_64") \
    && curl -fsSL \
        "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz" \
        -o /tmp/s6-noarch.tar.xz \
    && curl -fsSL \
        "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz" \
        -o /tmp/s6-arch.tar.xz \
    && tar -C / -Jxpf /tmp/s6-noarch.tar.xz \
    && tar -C / -Jxpf /tmp/s6-arch.tar.xz \
    && rm /tmp/*.tar.xz

# Install Node.js (LTS) and the claude CLI via npm.
# We use the NodeSource setup script to get a recent LTS release.
ARG NODE_VERSION=20
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && npm install -g @anthropic-ai/claude-code \
    && rm -rf /var/lib/apt/lists/*

# Copy Go binaries from builder
COPY --from=go-builder /build/bin/controller /usr/local/bin/controller
COPY --from=go-builder /build/bin/oasis      /usr/local/bin/oasis

# Copy built Next.js static assets
COPY --from=webapp-builder /build/dist/webapp /srv/webapp

# Copy s6-overlay service definitions
COPY etc/s6-overlay/ /etc/s6-overlay/

# Copy default NGINX stub config (controller overwrites this after tsnet setup)
COPY etc/nginx/nginx.conf /etc/nginx/nginx.conf

# Make s6 run scripts executable
RUN chmod +x /etc/s6-overlay/s6-rc.d/controller/run \
             /etc/s6-overlay/s6-rc.d/nginx/run

# Create non-root user (uid 1000)
RUN groupadd -g 1000 oasis \
    && useradd -u 1000 -g oasis -s /sbin/nologin -d /data oasis \
    && mkdir -p /data/db /data/ts-state /data/agent-runs \
    && chown -R oasis:oasis /data /srv/webapp

# Allow the oasis user to write NGINX config and use NGINX temp/log dirs
RUN chown -R oasis:oasis /etc/nginx /var/lib/nginx /var/log/nginx

# The management API is published loopback-only: 127.0.0.1:04515:04515
EXPOSE 04515

USER 1000

ENTRYPOINT ["/init"]
