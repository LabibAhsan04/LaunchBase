# Security and dependency audit (Phase 11)

Reviewed against the portfolio single-host threat model in `docs/security.md`.

## Controls verified in this release

- GitHub OAuth allowlist (`AUTH_ALLOWLIST`) blocks uninvited accounts
- Production compose forces `AUTH_MODE=github`
- Postgres/Redis are not published on the host in production
- Only Traefik publishes 80/443; bootstrap firewall matches
- Worker alone mounts the Docker socket; user containers do not
- Global READY/build caps + cleanup script for cost control
- Ops status endpoint requires `OPS_TOKEN` (returns 404 if unset)
- Secrets via `.env` + mounted PEM; examples are gitignored patterns

## Dependency posture

- Go module: stdlib-heavy API/worker; pin via `go.sum` (CI `go test` / image build)
- Web: Next.js / React via `pnpm-lock.yaml` (CI lint, typecheck, test, build)
- Base images: `postgres:16-alpine`, `redis:7-alpine`, `traefik:v3.6`, `golang:1.25-alpine`, `node:22-alpine`, distroless API, `docker:27-cli` worker

Re-run before each public demo:

```bash
cd backend && go list -m all
cd apps/web && pnpm audit
docker compose -f infrastructure/production/docker-compose.yml config
```

## Known limitations (do not oversell)

1. Compromised worker or Traefik implies host Docker control.
2. User container egress is not fully locked down on a typical Docker bridge.
3. Allowlist is login-based, not org SSO.
4. HTTP-only nip.io mode is for short demos — prefer HTTPS + real DNS.
5. Cleanup script marks STOPPED and removes containers; it does not replace a full capacity planner.

## Residual risk acceptance

Safe for **recruiter demos on a VM you control**. Not marketed as a public commercial hosting service.
