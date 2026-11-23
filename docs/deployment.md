# Production deployment (single Linux VM)

LaunchBase portfolio demos run on **one controlled VM**. This is not a multi-tenant commercial host. Anonymous arbitrary-code deployment stays disabled.

## Layout

| Piece | Network | Notes |
| --- | --- | --- |
| Postgres, Redis | `launchbase` only | No published host ports |
| API, web | `launchbase` + `launchbase-runtime` | Traefik routes by `CONTROL_HOST` |
| Worker | both + Docker socket | Only the worker may talk to the Docker daemon |
| User app containers | `launchbase-runtime` only | Never receive the socket or infra credentials |
| Traefik | `launchbase-runtime` | Publishes 80/443; Docker provider for app hostnames |

Important paths:

- `infrastructure/production/docker-compose.yml`
- `infrastructure/production/.env.example`
- `infrastructure/scripts/bootstrap-host.sh`
- `infrastructure/scripts/backup-db.sh` / `restore-db.sh`
- `infrastructure/scripts/cleanup-deployments.sh`

## Fresh VM

```bash
sudo bash infrastructure/scripts/bootstrap-host.sh
sudo mkdir -p /opt/launchbase && sudo chown "$USER" /opt/launchbase
git clone <your-fork-url> /opt/launchbase
cd /opt/launchbase/infrastructure/production
cp .env.example .env
# edit .env — AUTH_MODE=github, AUTH_ALLOWLIST, secrets, DNS names
sudo install -m 600 /path/to/github-app.pem /etc/launchbase/github-app.pem
```

Firewall (from bootstrap): **SSH, HTTP, HTTPS only**.

### DNS

- `CONTROL_HOST` → VM (dashboard + API)
- `*.BASE_DOMAIN` → VM (user apps, e.g. `demo.apps.example.com`)

### HTTPS (preferred)

```bash
docker compose --env-file .env up -d --build
```

Let's Encrypt HTTP-01 uses port 80. Dashboard/API serve on **443**.

### Temporary HTTP demo (no TLS)

Use only on a short-lived VM or when you lack a domain (e.g. `CONTROL_HOST=<ip>.nip.io`):

```bash
docker compose -f docker-compose.yml -f docker-compose.http.yml --env-file .env up -d --build
```

Do not present HTTP-only mode as a production security posture.

## Access control

- `AUTH_MODE=github` (required in compose)
- `AUTH_ALLOWLIST=yourlogin,teammate` — OAuth users outside the list get `403 not_invited`
- No open registration; GitHub App installation still gates which repos can deploy
- Set `ENV_ENCRYPTION_KEY` (or rely on `SESSION_SECRET`) so project env vars encrypt at rest; rotating it without re-saving vars makes ciphertext unreadable

## Operator status

```bash
export OPS_TOKEN=... API_PUBLIC_URL=https://CONTROL_HOST
infrastructure/scripts/ops-status.sh
# or
curl -H "Authorization: Bearer $OPS_TOKEN" https://CONTROL_HOST/api/v1/ops/status
```

Returns queue depth, ready/building/failed counts, and disk usage for `OPS_DISK_PATH` (default `/`).

## Backups

```bash
sudo BACKUP_DIR=/var/backups/launchbase infrastructure/scripts/backup-db.sh
sudo infrastructure/scripts/restore-db.sh /var/backups/launchbase/launchbase-….sql.gz
```

Suggested cron (root):

```cron
15 2 * * * /opt/launchbase/infrastructure/scripts/backup-db.sh >>/var/log/launchbase-backup.log 2>&1
*/30 * * * * /opt/launchbase/infrastructure/scripts/cleanup-deployments.sh >>/var/log/launchbase-cleanup.log 2>&1
```

Cleanup stops the oldest READY deployments when `MAX_READY_GLOBAL` is exceeded, removes exited `launchbase-*` containers, and prunes dangling images.

## Cost caps

Set in `.env`: `MAX_BUILDS_*`, `MAX_READY_PER_USER`, `MAX_READY_GLOBAL`, container memory/CPU/PIDs. The API rejects new deploys when caps are hit (`429`).

## GitHub App URLs

Use HTTPS public URLs:

- Callback: `https://CONTROL_HOST/api/v1/auth/callback`
- Webhook: `https://CONTROL_HOST/api/v1/webhooks/github`
- Homepage: `https://CONTROL_HOST`

## Smoke checklist

1. `curl -fsS https://CONTROL_HOST/ready`
2. Sign in with an allowlisted GitHub user
3. Install the App on a demo repo; create project; deploy
4. Open `https://<slug>.BASE_DOMAIN/`
5. `ops-status.sh` shows queue/ready metrics
6. Run backup then a restore on a disposable VM once

## Honest limitations

- Single host: Docker escape or Traefik/socket compromise affects the demo
- Worker holds the Docker socket by design
- User containers still have typical NAT egress unless you add a custom egress proxy
- Not a public PaaS — allowlist + GitHub App installation only
