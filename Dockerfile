# Multi-stage production Dockerfile for Breeze.
# Builds the embedded UI + Go binary, then ships a minimal runtime image.

# ---- Build stage ----
FROM golang:1.26-bookworm AS build

RUN apt-get update && apt-get install -y --no-install-recommends \
    git make ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY . .

# Deno is required for the UI build (Vite). Pin the version and verify the
# checksum so builds are reproducible and supply-chain attacks are harder.
ARG DENO_VERSION=2.9.5
ARG DENO_SHA256=8b010a3b1a4a0188a67cdb8a7a27348b2a501af78aec7fc74f2ace167368d530
RUN apt-get install -y --no-install-recommends unzip \
    && curl -fsSL https://dl.deno.land/release/v${DENO_VERSION}/deno-x86_64-unknown-linux-gnu.zip -o /tmp/deno.zip \
    && echo "${DENO_SHA256}  /tmp/deno.zip" | sha256sum -c - \
    && unzip -q /tmp/deno.zip -d /usr/local/bin \
    && rm /tmp/deno.zip \
    && apt-get purge -y unzip \
    && apt-get autoremove -y \
    && deno --version

# Build the full binary (UI embed + Go). The .git directory is excluded from
# the build context, so `git describe` cannot run; inject the version via a
# build arg instead (e.g. --build-arg GIT_VERSION="$(git describe --tags)").
ARG GIT_VERSION=dev
RUN cd ui && deno task build
RUN go build -ldflags "-X ipmanlk/breeze/internal/version.Version=${GIT_VERSION}" -o bin/breeze ./cmd/server

# ---- Runtime stage ----
FROM debian:bookworm-slim

# sqlite3 is included for debugging; ca-certificates for outbound TLS (SMTP,
# web push, TURN). The binary embeds modernc.org/sqlite (pure Go, no CGO).
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates sqlite3 wget \
    && rm -rf /var/lib/apt/lists/*

# The binary serves the embedded UI; it only needs the data dir for SQLite +
# uploads. /data is the volume mount point. Create it owned by the runtime
# user so a fresh named volume or bind mount is writable on first start.
RUN groupadd --system --gid 1001 breeze \
    && useradd --system --uid 1001 --gid breeze --no-create-home --home-dir /data breeze \
    && mkdir -p /data \
    && chown -R breeze:breeze /data
WORKDIR /data
COPY --from=build /src/bin/breeze /usr/local/bin/breeze

ENV PORT=8080 \
    DB_PATH=/data/breeze.db \
    UPLOAD_DIR=/data/uploads

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

EXPOSE 8080
USER breeze
VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/breeze"]
