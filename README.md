# LaunchBase

LaunchBase takes a GitHub repository, builds it in Docker, and gives you a URL. The API stores projects and deployments in Postgres. Creating a deployment from the API runs the worker in that same request and saves the result. Redis is not in that path yet.

## What is on disk

| Path | What it is right now |
| --- | --- |
| `apps/web` | Next.js page. Placeholder, not a dashboard. |
| `backend/cmd/api` | Go HTTP server. Health check, projects, and deployments. |
| `backend/cmd/worker` | Deploy command. Clones a public GitHub repo, builds its Dockerfile, and prints a localhost URL. |
| `docker-compose.yml` | Postgres, Redis, Traefik on a network named `launchbase`. |
| `docs/openapi.yaml` | HTTP contract. |

The API process opens Postgres and runs migrations on startup. Handlers do not call Docker. A service does, by calling the same deploy function the worker command uses. That call is synchronous for now. The next phase puts it on a Redis queue so the HTTP response can return while the build is still running.

There is no login yet. Every request is the seeded user `local` (`00000000-0000-4000-8000-000000000001`).

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
make lint
make format
```

`make test` runs `go test ./...` and `pnpm typecheck`. The API tests need Postgres (`make dev-infra`). They do not need a Docker build. The example deploy scripts do. `make lint` is `go vet` plus eslint.

## API

Start Postgres, then the API. Migrations run as the process starts.

```bash
make dev-infra
make api
```

Create a project and read it back:

```bash
curl -s -X POST http://localhost:8080/api/v1/projects \
  -H 'content-type: application/json' \
  -d '{"name":"Demo","repoUrl":"https://github.com/acme/web","branch":"main"}'

curl -s http://localhost:8080/api/v1/projects
```

`repoUrl` is stored as `https://github.com/acme/web.git`. SSH URLs and local paths are rejected with `400` and a JSON `error` object.

A deployment is created under a project. This call does not return until the build has finished. Build lines print in the API terminal. Timeouts and container limits come from the same variables as `make worker` (`CLONE_TIMEOUT`, `BUILD_TIMEOUT`, `START_TIMEOUT`, `DEPLOYMENT_MEMORY_LIMIT`, `DEPLOYMENT_CPU_LIMIT`, `DEPLOYMENT_PIDS_LIMIT`). `make api` does not load `.env` by itself. With those variables unset, the defaults are 1 minute to clone, 10 minutes to build, 1 minute to start, 512m memory, 1 CPU, and 128 processes.

```bash
curl -s -X POST http://localhost:8080/api/v1/projects/PROJECT_ID/deployments \
  -H 'content-type: application/json' \
  -d '{"branch":"main"}'
```

The body comes back `READY` with a localhost `url`, or `FAILED` with `error`. Stop a ready deployment with:

```bash
curl -s -X POST http://localhost:8080/api/v1/deployments/DEPLOYMENT_ID/stop
```

Stopping anything that is not `READY` returns `409`. The status list is the same one the worker uses. `QUEUED` cannot jump to `READY`. The contract is `docs/openapi.yaml`.

## Not built yet

The queue, GitHub login, live logs, and the real dashboard are later work. Until the queue exists, one slow build holds the API request that started it.
