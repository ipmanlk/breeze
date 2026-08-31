# Configuration Reference

All configuration is done via environment variables. No config files needed. A
`.env.example` is provided at the project root for reference.

## Required

| Variable     | Description                                                                                                               |
| ------------ | ------------------------------------------------------------------------------------------------------------------------- |
| `JWT_SECRET` | Secret key for signing JWT tokens. Must be a random string (at least 32 chars). If missing, the app will refuse to start. |

## Environment

| Variable  | Default       | Description                                                                                                                                   |
| --------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `APP_ENV` | `development` | Runtime environment. `development` enables CORS for the Vite dev server at `http://localhost:5173`. `production` uses strict same-origin, no CORS. |

## Optional

Boolean environment variables (e.g. `TURN_ENABLED`) are parsed via Go's
`strconv.ParseBool`, which accepts `1`/`0`, `t`/`f`, `true`/`false`, and
`True`/`False` (case-insensitive). Empty or unparseable values fall back to
the documented default.

| Variable          | Default            | Description                                                                                   |
| ----------------- | ------------------ | --------------------------------------------------------------------------------------------- |
| `PORT`            | `8080`             | HTTP listen port number (no colon). The server listens on `:<PORT>`, which binds all interfaces. Do not set `:3000`. |
| `DB_PATH`         | `./data/plume.db` | Path to the SQLite database file. Created automatically. Use an absolute path for production. |
| `UPLOAD_DIR`      | `./data/uploads`   | Directory for uploaded file attachments. Created automatically.                               |
| `MAX_UPLOAD_SIZE` | `52428800` (50MB)  | Maximum file upload size in bytes.                                                            |
| `CORS_ORIGINS`    | `http://localhost:5173,http://localhost:4173` (dev) | CORS allowed origins (comma-separated). Empty/absent = dev defaults. In production set to your SPA origin(s). Cookies use `SameSite=Lax` so same-origin deployments do not need CORS. |
| `COOKIE_SAME_SITE` | `lax` | Cookie SameSite attribute: `lax`, `none`, or `strict`. Set to `none` for cross-origin deployments (SPA and API on different origins); requires `Secure` (always enabled). |
| `TRUSTED_PROXY_CIDRS` | (empty) | Comma-separated CIDR ranges of trusted reverse proxies (e.g. `10.0.0.0/8,172.16.0.0/12`). When set, `X-Forwarded-For`/`X-Real-IP` are only honored from these IPs for client-IP detection and rate limiting. When empty, `r.RemoteAddr` is used directly (secure default). |
| `LOG_LEVEL`       | `debug` in development, `info` otherwise | Logging level: `debug`, `info`, `warn`, `error`. Unset or unknown values fall back by `APP_ENV`. |
| `STUN_URLS`       | `stun:stun.l.google.com:19302` | Comma-separated list of STUN servers for WebRTC NAT traversal in voice channels. |
| `TURN_ENABLED`    | `false`            | Enable a TURN relay server for voice channels (for clients behind restrictive NATs). Accepts standard boolean forms: `1`/`0`, `true`/`false`, `True`/`False` (parsed via `strconv.ParseBool`). |
| `TURN_HOST`       | `localhost`        | TURN server hostname (used to build a plain UDP `turn:host:port` URL when `TURN_URLS` is not set). |
| `TURN_PORT`       | `3478`             | TURN server port (used with `TURN_HOST`). |
| `TURN_URLS`       | (empty)            | Comma-separated explicit TURN URI(s) to advertise to clients, overriding `TURN_HOST`/`TURN_PORT`. Supports TCP/TLS transports for restrictive firewalls, e.g. `turn:host:3478,turn:host:3478?transport=tcp,turns:host:443?transport=tcp`. Ephemeral credentials from `TURN_SECRET` (or static `TURN_USER`/`TURN_PASS`) are applied to every URL. |
| `TURN_USER`       | (empty)            | TURN server username (long-term credentials). |
| `TURN_PASS`       | (empty)            | TURN server password (long-term credentials). |
| `TURN_SECRET`     | (empty)            | Shared secret for ephemeral TURN REST credentials (RFC 5766 / coturn `use-auth-secret`). When set, each voice join receives per-user, time-limited credentials instead of the static `TURN_USER`/`TURN_PASS`. |
| `TURN_CREDENTIAL_TTL` | `12h`          | Lifetime of ephemeral TURN credentials (Go duration syntax, e.g. `6h`, `30m`). |
| `VOICE_MAX_PARTICIPANTS` | `25`          | Maximum concurrent participants per voice channel. |

## WebSocket

WebSocket connections are bounded by per-user and global caps to protect
against resource exhaustion, and typing indicators are debounced server-side
to prevent broadcast amplification. All optional with sane defaults; `0`
disables the corresponding limit.

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `WS_MAX_CONNECTIONS_PER_USER` | `10` | Maximum concurrent WS connections per user (across tabs/devices). `0` = unlimited. |
| `WS_MAX_CONNECTIONS_GLOBAL` | `5000` | Maximum total concurrent WS connections server-wide. `0` = unlimited. |
| `WS_TYPING_DEBOUNCE` | `1s` | Minimum interval between re-broadcasting `typing_start` for the same user+conversation. `0` = no debounce. |

## Audit log retention

Audit log entries can be purged automatically after a configurable retention
period (H8). Both variables use Go duration syntax (`30m`, `6h`, `720h`, …).
When `AUDIT_RETENTION` is unset or `0`, entries are kept forever and no purge
runs.

| Variable                 | Default | Description                                                                        |
| ------------------------ | ------- | ---------------------------------------------------------------------------------- |
| `AUDIT_RETENTION`        | (empty) | How long audit log entries are kept before purge (Go duration, e.g. `720h` = 30 days). Unset/`0` = keep forever. |
| `AUDIT_CLEANUP_INTERVAL` | `6h`    | How often the retention purge runs. Only used when `AUDIT_RETENTION` is set.       |

## Email delivery (SMTP)

Outbound email is **optional**. When `SMTP_HOST` is unset the mailer is a
no-op (air-gapped friendly): password-reset links are logged server-side,
invite tokens are returned in the API response for manual sharing, and the
email-notifications user preference has no effect.

When `SMTP_HOST` is set, Plume sends transactional email (password reset,
invites, and per-notification email copies when the recipient has email
notifications enabled) via stdlib `net/smtp`, with no external Go
dependencies.

| Variable         | Default   | Description                                                                 |
| ---------------- | --------- | -------------------------------------------------------------------------- |
| `SMTP_HOST`      | (empty)   | SMTP server hostname. Empty = email disabled (air-gapped mode).            |
| `SMTP_PORT`      | `587`     | SMTP port. `587` = STARTTLS, `465` = implicit TLS.                          |
| `SMTP_USER`      | (empty)   | SMTP username. Empty = no auth (e.g. local relay).                         |
| `SMTP_PASS`      | (empty)   | SMTP password.                                                             |
| `SMTP_FROM`      | `SMTP_USER` | Sender address. Falls back to `SMTP_USER` when unset.                    |
| `SMTP_FROM_NAME` | `Plume`  | Sender display name.                                                       |
| `APP_URL`        | (empty)   | Public base URL for links in emails (e.g. `https://plume.example.com`). Falls back to the request `Host` for password-reset logging when empty. |

## Browser push notifications (Web Push)

Browser push is **optional**. When `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY` are
unset, Web Push is disabled: the UI hides the opt-in and the existing
in-page `Notification` API (wired in §4) still covers the tab-open case.

When configured, Plume registers a service worker, subscribes via the
Push API, and sends RFC 8291-encrypted payloads on `notification_new` when
the recipient has `desktop_notifications` enabled. Pushes are delivered by
the browser even when the tab is closed.

Generate a key pair once:

```bash
go run ./cmd/vapidkeys
```

| Variable           | Default                    | Description                                                                 |
| ------------------ | -------------------------- | --------------------------------------------------------------------------- |
| `VAPID_PUBLIC_KEY`  | (empty)                    | P-256 public key (base64url). Empty = push disabled.                        |
| `VAPID_PRIVATE_KEY` | (empty)                    | P-256 private key (base64url). Must pair with the public key.               |
| `VAPID_SUBJECT`     | `mailto:noreply@plume.local` | Contact URL/mailto included in the VAPID JWT (RFC 8292).                  |

## Example (development)

```bash
export JWT_SECRET="dev-secret-not-for-production-use"
export APP_ENV="development"

./plume
```

## Example (production)

```bash
export JWT_SECRET="your-secret-key-here-at-least-32-chars"
export APP_ENV="production"
export PORT="3000"
export DB_PATH="/var/lib/plume/plume.db"
export UPLOAD_DIR="/var/lib/plume/uploads"

./plume
```

Set `CORS_ORIGINS` to your domain if the SPA and API are served from different
origins. When `CORS_ORIGINS` is set, Plume applies the configured allowlist in
both development and production. If `CORS_ORIGINS` is empty in production, the
server defaults to same-origin behavior (no CORS headers), which is safe for the
common case where the SPA and API share an origin.

## Reverse Proxy

When running behind nginx or Caddy:

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;

    # WebSocket support (chat, presence, voice signaling, live notifications)
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 86400;
}
```

The `Upgrade`/`Connection` headers are required for the WebSocket endpoint
(`/ws`). Without them, realtime features silently fail behind nginx.

## Auth

Plume uses JWT (HS256) for session tokens, stored in a `__Host-token` HttpOnly
cookie with `SameSite=Lax`. The cookie is only sent over HTTPS (`Secure` flag).
Login is rate-limited to 10 attempts per 10 minutes per IP.

