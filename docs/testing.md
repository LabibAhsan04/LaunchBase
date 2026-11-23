# Testing

LaunchBase tests sit in three layers. Prefer the lowest layer that can catch the bug.

## Layers

| Layer | What it covers | Command |
| --- | --- | --- |
| Unit | State machine, parsing, redaction, middleware, observability | `cd backend && go test ./...` (no Docker build). Frontend: `cd apps/web && pnpm test` |
| Integration | Postgres + Redis API/queue paths; optional Docker example deploys | Needs `make dev-infra`. Tagged Docker examples: `go test -tags=integration ./internal/deployments -run TestExampleNodeDemo` |
| End-to-end | Example app built and reachable on a Traefik hostname | `infrastructure/scripts/e2e-deploy.sh` or the `e2e` GitHub Actions workflow |

## Local commands

```bash
make test          # go test, web typecheck, web vitest
make lint          # go vet, eslint
make coverage      # go test with coverage summary
make e2e           # Traefik + node-demo routed URL check (needs Docker)
```

API and queue integration tests skip or fail cleanly when Postgres/Redis are down. Docker example tests require Docker Desktop and the `launchbase-runtime` network from compose.

## CI matrix

| Job | Checks |
| --- | --- |
| `backend` | gofmt, go vet, `go test` with coverage, migrate + `/ready` smoke |
| `web` | pnpm install (frozen lockfile), lint, typecheck, test, build |
| `docker` | multi-stage `backend/Dockerfile` targets `api` and `worker` |
| `e2e` | compose Traefik + `TestExampleNodeDemo` |

Caches: Go module cache via `setup-go` + `go.sum`; pnpm store via `setup-node` cache. Builds stay deterministic with frozen lockfiles and `-count=1` on Go tests.

## Coverage

`make coverage` writes `backend/coverage.out` and prints the total. Use coverage to find untested packages, not to hit an arbitrary percentage by asserting implementation details.
