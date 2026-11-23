# Release checklist

Use before tagging a release or promoting a demo environment.

## Pre-release

- [ ] `main` CI green (`backend`, `web`, `docker`, `e2e`)
- [ ] `docs/openapi.yaml` version and paths match the running API
- [ ] Migrations apply cleanly on an empty database and on the previous schema
- [ ] `AUTH_MODE=github` + `AUTH_ALLOWLIST` for any shared host; `AUTH_MODE=dev` only for local
- [ ] Secrets present via env/files only (no `.env` or PEM in git)
- [ ] Container limits, `MAX_READY_GLOBAL`, and rate limits reviewed (`docs/security.md`)
- [ ] Production compose / DNS / TLS smoke from `docs/deployment.md`
- [ ] Backup script run once; restore tested on a disposable VM
- [ ] `docs/demo.md` walkthrough rehearsed; screenshots updated if UI changed
- [ ] `docs/audit.md` limitations still accurate

## Smoke after deploy

- [ ] `GET /health` → `ok` on API and worker
- [ ] `GET /ready` → `ok` (Postgres + Redis reachable)
- [ ] `GET /metrics` returns Prometheus text
- [ ] Sign-in (or dev mode) and create a project from an allowed repository
- [ ] Queue a deployment; logs stream; status reaches `READY` or a clear `FAILED`
- [ ] Routed URL responds through Traefik
- [ ] Stop / redeploy / rollback behave as documented
- [ ] SIGTERM: API drains HTTP; worker finishes or releases the current job and `/ready` becomes unavailable

## Rollback

- [ ] Previous container images or binaries identified
- [ ] Database migrations are forward-only; if a migration is unsafe to leave applied, document a manual repair before shipping it
