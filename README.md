# LaunchBase

LaunchBase is a portfolio-grade mini deployment platform: connect a GitHub repo, watch live build logs, and get a Traefik-routed URL to a hardened Docker container.

**Problem.** Shipping a side project still means gluing together git, Docker, a reverse proxy, and logs. LaunchBase demonstrates that path as one coherent system you can run locally or on a single demo VM.

**Not a public PaaS.** Demo access is GitHub OAuth + an allowlist. Anonymous arbitrary-code hosting is intentionally out of scope (`docs/audit.md`).

## Architecture

```text
Browser → Next.js dashboard → Go API → Postgres
                              ↓ outbox
                            Redis stream → Worker → Docker build/run
                                              ↓
                                         Traefik (*.BASE_DOMAIN)
```

Invariants: the API never talks to Docker; only the worker does. User containers never receive the Docker socket. State transitions go through one validated machine. See `docs/adr/`.

## Feature demo

Two-minute script: `docs/demo.md`. Screenshots belong in `docs/screenshots/`.

## Engineering challenges

- Durable queue via Postgres outbox + Redis (survives Redis blips)
- Idempotent worker locks and stable container names on retry
- Live SSE logs with secret redaction
- Traefik routing on a network isolated from Postgres/Redis
- CI that builds images and deploys `examples/node-demo` through Traefik

## Security model

Trust assumptions, container hardening, rate limits, and accepted single-host risks: `docs/security.md`. Vulnerability reports: `SECURITY.md`.

## Public demo VM

Vendor-neutral Compose stack, HTTPS, firewall, backups, cleanup, and ops status: **`docs/deployment.md`**.

## What is on disk

| Path | What it is |
| --- | --- |
| `apps/web` | Next.js dashboard |
| `backend/cmd/api` | Go HTTP API |
| `backend/cmd/worker` | Deployment worker |
| `docker-compose.yml` | Local Postgres, Redis, Traefik |
| `infrastructure/production/` | Single-VM production Compose + env example |
| `docs/openapi.yaml` | HTTP contract |
| `docs/testing.md` / `docs/observability.md` | CI and telemetry |
| `docs/deployment.md` / `docs/demo.md` | Portfolio launch |
| `docs/adr/` | Architecture decisions |

The API process opens Postgres, runs migrations, and publishes jobs to Redis. It does not build or start a user container. `POST /api/v1/projects/{id}/deployments` commits a `QUEUED` deployment and an outbox row in one transaction, then returns. If Redis is down, the row stays in `deployment_outbox` until the API or the worker can publish it.

`make worker` is a second process. It publishes any unpublished outbox rows, reads the Redis stream `launchbase:jobs`, locks that deployment, and runs the same build used by `make worker-once`. Stopping the API leaves the worker and its current build alone. A worker that dies mid-job leaves the Redis message unacked. The next worker claims it, removes the container named `launchbase-<id>` if one was left behind, and runs that deployment again. A second copy of the same job does not start a second container.

A clone or Docker connection problem is retried up to 3 times. A build failure, a health-check failure, and an illegal status jump are stored as `FAILED` and copied to the stream `launchbase:jobs:dead`. They are not retried.

With `AUTH_MODE=dev` (default), requests without a session cookie act as the seeded user `local` (`00000000-0000-4000-8000-000000000001`). With `AUTH_MODE=github`, project and deployment APIs require a signed-in session.

## Prerequisites

- Docker Desktop, running
- Node.js 22 or newer, and pnpm 10 (`corepack enable && corepack prepare pnpm@10.15.0 --activate`)
- Go 1.25 or newer

## Setup

```bash
cp .env.example .env
cp apps/web/.env.example apps/web/.env.local
make dev-infra
```

Compose does not read `DATABASE_URL` from `.env`. The Postgres user, password, and database are set in `docker-compose.yml` and they match the URL in `.env.example`. If `DATABASE_URL` is unset, the API uses that same local URL, including port 5433.

Three terminals for the API, the worker, and the dashboard:

```bash
make api
make worker
make web
```

- Dashboard: http://localhost:3000
- API health / ready / metrics: http://localhost:8080/health , `/ready`, `/metrics`
- Worker health / ready / metrics: http://localhost:8081/health , `/ready`, `/metrics`
- Traefik dashboard: http://localhost:8088

Optional Prometheus and Grafana (not started by `make dev-infra`):

```bash
make observability-up
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (admin / admin)

See `docs/observability.md`.

The browser talks to the Go API at `NEXT_PUBLIC_API_URL` (default `http://localhost:8080`). The API allows that origin through `WEB_ORIGIN` (default `http://localhost:3000`).

Stop the containers with `make stop-infra`. That keeps the named volumes, so Postgres data sticks around.

## Deploy a Dockerfile repo

The repository has to be public, on `https://github.com/owner/name`, and it needs a `Dockerfile` at the root. SSH URLs and local paths are rejected.

```bash
REPO=https://github.com/owner/name BRANCH=main make worker-once
```

`BRANCH` can be left off. The command clones with `--depth 1`, builds an image named `launchbase/<id>`, and starts a container named `launchbase-<id>`. The container is not privileged. It gets a memory limit, a CPU limit, a process limit, a read-only root filesystem, and a writable `/tmp`. It joins the `launchbase-runtime` network with Traefik labels (not the Postgres/Redis network) and is not published on the host.

When the app answers through Traefik, the worker prints:

```text
id=...
image=launchbase/...
container=launchbase-...
url=http://app.launchbase.localhost/
```

Project URLs use `{slug}.launchbase.localhost` on port 80. A deployment-specific host `{slug}-{shortid}.launchbase.localhost` is used for the health check. If the build or the health check fails, the worker removes the container and the image, deletes the temp clone, and exits non-zero.

Ctrl-C cancels the clone or the build. A container that was already started is removed.

Repos without a Dockerfile are detected from `package.json` and the lockfile. The rules are in `docs/supported-frameworks.md`. The worker writes a Dockerfile of its own and builds that. It does not run npm, pnpm, or yarn on your machine.

The example in `examples/node-demo` is not a GitHub repo, so the worker command will not accept it as `--repo`. These scripts build the examples with the same code and then remove the container:

```bash
infrastructure/scripts/test-local-deploy.sh
infrastructure/scripts/test-node-http.sh
infrastructure/scripts/test-vite-demo.sh
infrastructure/scripts/test-next-demo.sh
```

`node-demo` already has a Dockerfile. `node-http` is a Node server without one. Vite and Next.js are detected. The scripts need Docker. `make test` does not run them.

## Checks

```bash
make test
make coverage
make lint
make format
make e2e
```

`make test` runs `go test ./...`, `pnpm typecheck`, and `pnpm test` in `apps/web`. The API and queue tests need Postgres and Redis (`make dev-infra`). They do not need a Docker build. `make e2e` and the example deploy scripts need Docker and Traefik. `make lint` is `go vet` plus eslint. Layers and the CI matrix are in `docs/testing.md`.

## Security

Trust model, container hardening, concurrency limits, and accepted single-host risks are in `docs/security.md`. Vulnerability reports: `SECURITY.md`. Use `AUTH_MODE=github` outside local development so deploys require a signed-in user and an installed repository.

## API

Start Postgres, then the API. Migrations run as the process starts.

```bash
make dev-infra
make api
```

Create a project and read it back (`AUTH_MODE=dev` needs no cookie):

```bash
curl -s -X POST http://localhost:8080/api/v1/projects \
  -H 'content-type: application/json' \
  -d '{"name":"Demo","repoUrl":"https://github.com/acme/web","branch":"main"}'

curl -s http://localhost:8080/api/v1/projects
```

Prefer `fullName` when the repo is linked through the GitHub App (`{"name":"Demo","fullName":"acme/web"}`). `repoUrl` is stored as `https://github.com/acme/web.git`. SSH URLs and local paths are rejected with `400` and a JSON `error` object.

A deployment is created under a project. The response comes back as soon as the row is saved, with `status` `QUEUED`. `make worker` has to be running for the build to start. Build lines print in the worker terminal. Timeouts and container limits come from the same variables as `make worker-once` (`CLONE_TIMEOUT`, `BUILD_TIMEOUT`, `START_TIMEOUT`, `DEPLOYMENT_MEMORY_LIMIT`, `DEPLOYMENT_CPU_LIMIT`, `DEPLOYMENT_PIDS_LIMIT`). `make api` and `make worker` do not load `.env` by themselves. With those variables unset, the defaults are 1 minute to clone, 10 minutes to build, 1 minute to start, 512m memory, 1 CPU, and 128 processes.

```bash
curl -s -X POST http://localhost:8080/api/v1/projects/PROJECT_ID/deployments \
  -H 'content-type: application/json' \
  -d '{"branch":"main"}'
```

Poll `GET /api/v1/deployments/DEPLOYMENT_ID` until `status` is `READY` or `FAILED`. A ready deployment has a Traefik `url` like `http://demo.launchbase.localhost/`. A failed one has `error`. Stop, redeploy, or roll back with:

```bash
curl -s -X POST http://localhost:8080/api/v1/deployments/DEPLOYMENT_ID/stop
curl -s -X POST http://localhost:8080/api/v1/deployments/DEPLOYMENT_ID/redeploy
curl -s -X POST http://localhost:8080/api/v1/deployments/DEPLOYMENT_ID/rollback
```

`redeploy` queues a new build from the project settings. `rollback` queues a new immutable deployment using that deployment's stored repo URL and branch. When a replacement becomes `READY`, the previous `READY` deployment is marked `SUPERSEDED` and its container is removed. Stopping anything that is not `READY` returns `409`. The contract is `docs/openapi.yaml`.

Build logs for a deployment:

```bash
curl -s "http://localhost:8080/api/v1/deployments/DEPLOYMENT_ID/logs"
curl -N "http://localhost:8080/api/v1/deployments/DEPLOYMENT_ID/logs/stream"
```

Lines are stored in Postgres with a per-deployment sequence number. Each deployment keeps at most 2000 lines. ANSI codes are stripped and values from `SESSION_SECRET`, `DATABASE_URL`, `REDIS_URL`, and anything listed in `LOG_REDACT_ENV` are replaced with `[redacted]` before a line is saved. The worker publishes new lines on Redis so the API can stream them. Heartbeats keep the stream open. A final `done` event closes it. `Last-Event-ID` resumes after a disconnect.

## Dashboard

Open http://localhost:3000 after `make web`. Pages:

- `/` project list
- `/login` GitHub sign-in
- `/install-github` GitHub App install explanation
- `/projects/new` create a project (repository and branch selectors when installed)
- `/projects/[projectId]` project detail and deploy form
- `/deployments/[deploymentId]` status, live logs, URL, Open/Stop/Redeploy/Roll back
- `/settings` account links

Status uses the same values as the API. The deployment page opens an SSE log stream (with cookies) and polls status every 10 seconds as a fallback.

Screenshots for the README: with the API and worker running, create a project, queue a deploy, open the deployment page while logs are scrolling, then again when it is `READY` or `FAILED`. Save the images under `docs/screenshots/` when you have them. That folder is not required for the app to run.

## GitHub App

Create a GitHub App for local development (do not commit the private key):

1. GitHub → Settings → Developer settings → GitHub Apps → New GitHub App.
2. Homepage URL: `http://localhost:3000`
3. Callback URL: `http://localhost:8080/api/v1/auth/callback`
4. Setup URL (optional): `http://localhost:3000/install-github`
5. Webhook URL: `http://localhost:8080/api/v1/webhooks/github` (use a tunnel such as ngrok for real delivery)
6. Webhook secret: set `GITHUB_WEBHOOK_SECRET` to the same value
7. Permissions: Repository → Metadata (read), Contents (read). Subscribe to `installation` and `installation_repositories`.
8. Where can this App be installed: Only on this account (or Any account for demos)
9. After create: note App ID, Client ID, generate a Client secret, generate a private key, set the slug

Put the values in `.env`:

```bash
AUTH_MODE=github
API_PUBLIC_URL=http://localhost:8080
GITHUB_APP_ID=
GITHUB_APP_SLUG=
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GITHUB_APP_PRIVATE_KEY_PATH=/absolute/path/to/private-key.pem
GITHUB_WEBHOOK_SECRET=
```

Sign in from `/login`, install from `/install-github`, then create a project from the repository selector. The worker mints a short-lived installation token for private clones and passes it as an HTTP Authorization header so it never lands in the stored remote URL or browser logs.
