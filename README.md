# LaunchBase

LaunchBase takes a GitHub repository, builds it in Docker, and gives you a URL. The worker can do that now for a public repository that already has a Dockerfile. The API still only answers a health check. Postgres, Redis, and Traefik are running for later phases and are not part of this deploy path.

## What is on disk

| Path | What it is right now |
| --- | --- |
| `apps/web` | Next.js page. Placeholder, not a dashboard. |
| `backend/cmd/api` | Go HTTP server. `GET /health` returns `{"status":"ok"}`. |
| `backend/cmd/worker` | Deploy command. Clones a public GitHub repo, builds its Dockerfile, and prints a localhost URL. |
| `docker-compose.yml` | Postgres, Redis, Traefik on a network named `launchbase`. |
| `backend/internal/*` | Empty packages. Filled in when that part of the system exists. |

The API does not open a database connection and it does not talk to Docker. Only the worker does. That split stays: later the API will record a deployment and hand it to the worker through Redis. Right now you run the worker yourself.

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

Two terminals, if you want the placeholder site and the health check:

```bash
make api
make web
```

- Web: http://localhost:3000
- Health: http://localhost:8080/health
- Traefik dashboard: http://localhost:8088

Stop the containers with `make stop-infra`. That keeps the named volumes, so Postgres data sticks around.

## Deploy a Dockerfile repo

The repository has to be public, on `https://github.com/owner/name`, and it needs a `Dockerfile` at the root. SSH URLs and local paths are rejected.

```bash
REPO=https://github.com/owner/name BRANCH=main make worker
```

`BRANCH` can be left off. The command clones with `--depth 1`, builds an image named `launchbase/<id>`, and starts a container named `launchbase-<id>`. The container is not privileged. It gets a memory limit, a CPU limit, a process limit, a read-only root filesystem, and a writable `/tmp`. Docker picks a free port on `127.0.0.1`.

When the app answers, the worker prints:

```text
id=...
image=launchbase/...
container=launchbase-...
url=http://127.0.0.1:<port>/
```

That URL is the app. Traefik is not in front of it yet. If the build or the health check fails, the worker removes the container and the image, deletes the temp clone, and exits non-zero.

Ctrl-C cancels the clone or the build. A container that was already started is removed.

The example in `examples/node-demo` is not a GitHub repo, so the worker command will not accept it as `--repo`. This script builds that example with the same code and then removes the container:

```bash
infrastructure/scripts/test-local-deploy.sh
```

It needs Docker. `make test` does not run it.

## Checks

```bash
make test
make lint
make format
```

`make test` runs `go test ./...` and `pnpm typecheck`. It does not need Docker. `make lint` is `go vet` plus eslint.

## Not built yet

Repos without a Dockerfile, the queue, auth, live logs, and the real dashboard are later work. Vite and Next.js examples are still empty.
