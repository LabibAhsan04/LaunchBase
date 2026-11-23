# LaunchBase security model

LaunchBase is a single-host portfolio platform. It can demonstrate layered defenses. It cannot safely run fully hostile untrusted internet code on a shared Docker host. This document describes assets, trust assumptions, controls that exist today, and accepted risks.

## Assets

| Asset | Why it matters |
| --- | --- |
| GitHub App private key / client secret | Can mint installation tokens and impersonate the app |
| Session cookies | Authenticate dashboard users |
| Postgres data | Projects, deployments, audit events, sessions |
| Redis | Job queue and live log fan-out |
| Docker engine | Build and run user repositories |
| Runtime containers | Untrusted application code |

## Trust assumptions

1. The operator controls the host, Docker daemon, `.env`, and GitHub App credentials.
2. `AUTH_MODE=dev` is for local development only. It may create projects from public URLs without an installation.
3. `AUTH_MODE=github` is required for any shared or demo environment: users must sign in, and deployments must use repositories granted through their GitHub App installation. Set `AUTH_ALLOWLIST` to invited GitHub logins so the portfolio demo is not open registration.
4. Repositories selected for deploy are treated as untrusted build input, even when the owner is authenticated.
5. Traefik may mount the Docker socket. User application containers must never receive it.

## Attack paths and mitigations

| Attack path | Mitigation |
| --- | --- |
| Steal GitHub App private key from git | Key path/env only; never committed; redacted from logs |
| Paste PAT into the UI | Not supported; App installation tokens only |
| Access another user's project | Session + `owner_id` filters on every project/deployment query |
| Escape a runtime container to the Docker socket | No socket mount; no privileged mode; `no-new-privileges`; `cap-drop ALL` |
| Reach Postgres/Redis from a user container | Separate `launchbase` (infra) and `launchbase-runtime` networks |
| Exhaust host CPU/memory/PIDs | Per-container `--cpus`, `--memory`, `--memory-swap`, `--pids-limit` |
| Hang the worker forever | Clone/build/start timeouts with cleanup |
| Flood the API | Per-IP rate limits; JSON body size cap (1 MiB) |
| Read secrets from build logs | ANSI strip + configured secret redaction |
| Inject LaunchBase env into apps | Runtime containers receive no LaunchBase env vars (MVP omits user env entirely) |

## Runtime container controls

Every ready deployment container is started with:

- network: `launchbase-runtime` only (not `launchbase`, not host networking)
- Traefik labels for routing
- `--read-only` root filesystem and `--tmpfs /tmp` (plus framework-specific tmpfs)
- `--security-opt no-new-privileges:true`
- `--cap-drop ALL`
- memory / CPU / PID limits
- non-root `--user` when the image defines one

Health checks run through Traefik hostnames after the container is on the runtime network.

## Timeouts and concurrency

| Control | Default env |
| --- | --- |
| Clone timeout | `CLONE_TIMEOUT=60s` |
| Build timeout | `BUILD_TIMEOUT=10m` |
| Start/health timeout | `START_TIMEOUT=60s` |
| Builds per user | `MAX_BUILDS_PER_USER=2` |
| Builds global | `MAX_BUILDS_GLOBAL=10` |
| Ready deployments per user | `MAX_READY_PER_USER=5` |
| API requests / minute / IP | `API_RATE_PER_MIN=120` |
| Auth requests / minute / IP | `AUTH_RATE_PER_MIN=20` |

## Environment variables for user apps

Encrypted-at-rest application secrets are deliberately omitted from the MVP. Deployment containers do not receive custom env files. Platform secrets never appear in API responses or Traefik labels.

## Outbound networking

Runtime containers share the Traefik/runtime bridge network and can make outbound connections from the host network namespace in a typical Docker Desktop setup. Fully isolating egress (egress proxy, network policies, or a custom bridge without NAT) is left for a production multi-tenant design. For this portfolio MVP, outbound remains allowed so common frameworks can fetch nothing at runtime while still working if they do; operators should treat that as an accepted risk.

## Storage limits

`DEPLOYMENT_STORAGE_LIMIT` is reserved for drivers that honor container storage quotas. Docker Desktop overlay2 often ignores `--storage-opt size=`. Image/layer growth is managed by cleaning superseded images after cutover and by keeping clone/build workdirs temporary.

## Audit events

The API writes audit rows for:

- `auth.login` / `auth.logout`
- `project.created`
- `deployment.created` / `deployment.stopped` / `deployment.redeployed` / `deployment.rollback`
- `deployment.ready` / `deployment.failed` / `deployment.superseded` (worker)
- `permission.denied`

## Accepted risks

1. Single shared Docker daemon: a kernel or Docker zero-day can affect the host.
2. Traefik holds a read-only Docker socket; compromise of Traefik is high impact.
3. Build containers temporarily use the host Docker engine; malicious Dockerfiles can attempt denial of service within configured limits.
4. `*.launchbase.localhost` has no TLS in local development.
5. Rate limits are in-process memory and reset on API restart.

## Reporting

See [SECURITY.md](../SECURITY.md).
