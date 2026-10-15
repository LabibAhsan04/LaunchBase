# LaunchBase

LaunchBase takes a GitHub repository, builds it in Docker, and gives you a URL. This checkout is only the local skeleton: a web app, a Go API, a worker process, and the infrastructure those will use later.

Nothing deploys yet. The API answers a health check. The worker starts and then sits there. Postgres, Redis, and Traefik come up in Docker and are not wired into the Go code.

## What is on disk

| Path | What it is right now |
| --- | --- |
| `apps/web` | Next.js page. Placeholder, not a dashboard. |
| `backend/cmd/api` | Go HTTP server. `GET /health` returns `{"status":"ok"}`. |
| `backend/cmd/worker` | Separate process. Logs that it is up, then waits for Ctrl-C. |
| `docker-compose.yml` | Postgres, Redis, Traefik on a network named `launchbase`. |
| `backend/internal/*` | Empty packages. Filled in when that part of the system exists. |

The API does not open a database connection. The worker does not talk to Redis or Docker. That split is intentional: later the API will record work, and only the worker will build containers.

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

Compose does not read `DATABASE_URL` from `.env`. The Postgres user, password, and database are set in `docker-compose.yml` and they match the URL in `.env.example`. The API only reads `API_ADDR` today.

Three terminals:

```bash
make api
make worker
make web
```

- Web: http://localhost:3000
- Health: http://localhost:8080/health
- Traefik dashboard: http://localhost:8088

Stop the containers with `make stop-infra`. That keeps the named volumes, so Postgres data sticks around.

## Checks

```bash
make test
make lint
make format
```

`make test` runs `go test ./...` and `pnpm typecheck`. It does not need Docker. `make lint` is `go vet` plus eslint.

## Not built yet

Cloning, image builds, the queue, auth, logs, and the real dashboard are later work. Empty folders under `examples/`, `docs/`, and `backend/internal/` are reserved for that.
