# Self-Hosting Setup Guide

## Prerequisites

- **Go 1.26+** (see `go.mod` for current version) OR download the pre-built binary
- 50MB disk space for the binary + database + uploads

## Option 1: Download Pre-built Binary

```bash
# Download the latest release
wget https://github.com/ipmanlk/plume/releases/latest/download/plume-linux-amd64
chmod +x plume-linux-amd64
mv plume-linux-amd64 /usr/local/bin/plume
```

## Option 2: Build from Source

```bash
git clone https://github.com/ipmanlk/plume.git
cd plume
make build          # Builds ui/dist then compiles Go binary
# Binary at: bin/plume
```

## Running

### Minimal Setup

```bash
export JWT_SECRET="$(openssl rand -base64 32)"
./plume
```

Visit `http://localhost:8080`. The setup wizard will guide you through creating
the first organization and admin account.

### Docker Compose

The repo ships a [`compose.yaml`](../../compose.yaml) that builds the image and
runs Plume with a named volume for the database and uploads:

```bash
# 1. Create your config from the template, then set a real JWT_SECRET (>= 32 chars)
cp .env.example .env
#    edit .env and set JWT_SECRET to a random secret, e.g. `openssl rand -base64 32`

# 2. Build and start (first run builds the image; the setup wizard is at :8080)
docker compose up -d

# Watch the first-boot log or check status
# docker compose logs -f plume
# docker compose ps
```

Key properties of the provided file:

- **No `version:` key**: the Compose spec treats it as obsolete; the file relies
  on the current spec automatically.
- **Non-root container**: the image runs as uid/gid 1001 and writes to `/data`.
- **Named volume `plume-data`** mounted at `/data`, so the SQLite database and
  uploads survive rebuilds and upgrades. Back it up like any other data dir
  (see [Backups](#backups)).
- **`APP_ENV=production`** is forced; `PORT`, `DB_PATH`, `UPLOAD_DIR` are wired
  to the container layout. Everything else comes from your `.env` via
  `env_file` (SMTP, VAPID, TURN, etc.).
- **Healthcheck** mirrors the Dockerfile so `docker compose ps` reports
  healthy/unhealthy and downstream services can gate on it.
- **`restart: unless-stopped`** so the service comes back after a reboot or
  crash (unless you explicitly stopped it).
- Expose a different host port by setting `PLUME_HTTP_PORT` in your
  environment before `docker compose up`.

To upgrade: `docker compose pull` (when using a prebuilt image) or
`docker compose build && docker compose up -d`. Data migrations run
automatically on startup.

### Systemd Service

Create `/etc/systemd/system/plume.service`:

```ini
[Unit]
Description=Plume Project Manager
After=network.target

[Service]
Type=simple
User=plume
Group=plume
# Must be a random secret of at least 32 characters. Generate one with:
#   openssl rand -hex 32
Environment="JWT_SECRET=change-me-min-32-chars-random-string-here"
Environment="PORT=8080"
Environment="DB_PATH=/var/lib/plume/plume.db"
Environment="UPLOAD_DIR=/var/lib/plume/uploads"
ExecStart=/usr/local/bin/plume
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo useradd -r -s /bin/false plume
sudo mkdir -p /var/lib/plume
sudo chown plume:plume /var/lib/plume
sudo systemctl daemon-reload
sudo systemctl enable --now plume
```

## Data Locations

| Data     | Default Path       |
| -------- | ------------------ |
| Database | `./data/plume.db` |
| Uploads  | `./data/uploads/`  |

Use absolute paths in production. The `data/` directory is created automatically
on first run.

## Backups

Use the built-in backup endpoint for a consistent, online snapshot:

```bash
# Requires an owner/admin API session (cookie auth).
curl -fsS -b cookies.txt \
  https://plume.example.com/api/backup/download \
  -o /backups/plume-$(date +%Y%m%d).db

# Backup uploads
rsync -av /var/lib/plume/uploads/ /backups/uploads/
```

The backup endpoint uses SQLite `VACUUM INTO`, which produces a consistent
copy without stopping the service. Copying only the `.db` file directly (e.g.
with `cp`) is **not recommended** in WAL mode because the live `-wal`/`-shm`
files may contain committed transactions that have not yet been checkpointed,
leading to an inconsistent or incomplete snapshot. If you must back up the
filesystem directly, stop Plume first or use `sqlite3 /path/db ".backup"`.

## Upgrading

1. Stop the service: `sudo systemctl stop plume`
2. Backup the database (see above)
3. Replace the binary: `cp plume /usr/local/bin/plume`
4. Start the service: `sudo systemctl start plume`

Database migrations run automatically on startup. Downgrades are not
supported; always back up before upgrading.
