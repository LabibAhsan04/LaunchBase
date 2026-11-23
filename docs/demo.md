# Two-minute recruiter demo

## Before you start (operator)

1. Production stack is up (`docs/deployment.md`).
2. You are on `AUTH_ALLOWLIST`.
3. GitHub App is installed on a small public demo repo (or `examples/node-demo` pushed to your account).
4. Optional: open `ops-status.sh` in a second window.

## Script (~2 minutes)

1. **Problem (15s)** — “Deploying a side project usually means wrangling VMs, Docker, and reverse proxies. LaunchBase is a mini Railway-style platform I built end-to-end.”
2. **Sign in (15s)** — Open the dashboard HTTPS URL → Sign in with GitHub (allowlisted). Show that random accounts are rejected if asked.
3. **Create project (20s)** — New project → pick the demo repo + branch → Create.
4. **Deploy + logs (40s)** — Deploy → open the deployment page → live SSE logs scroll through clone/build/start.
5. **URL (20s)** — When status is READY, click the Traefik URL (`https://<slug>.BASE_DOMAIN/`) and show the app response.
6. **Env (optional)** — On the project page, set an app key (masked after save), redeploy, show the app reading it.
7. **Engineering (20s)** — “API never talks to Docker; a worker owns builds. Postgres outbox + Redis queue, hardened containers, encrypted env injection.”
8. **Stop (10s)** — Stop the deployment to free the ready slot.

## Fallback if something fails

| Symptom | What to do live |
| --- | --- |
| OAuth error | Use a backup allowlisted account; check `API_PUBLIC_URL` callback URL on the GitHub App |
| Stuck QUEUED | `ops-status.sh` — if queue depth > 0, check `docker compose logs worker` |
| Build FAILED | Open logs on the page; common cause is missing Dockerfile / detect failure — switch to `examples/node-demo` |
| READY but URL dead | Confirm `*.BASE_DOMAIN` DNS and Traefik; show `/ready` + worker logs |
| Total outage | Show local `make test` / GitHub Actions greens and architecture in README |

## Screen recording checklist

- [ ] Dashboard home (signed in)
- [ ] Project create with repo selector
- [ ] Deployment page with scrolling logs
- [ ] Browser tab on the public app URL
- [ ] Optional: `ops/status` JSON (redact tokens)

Save stills under `docs/screenshots/` (`dashboard.png`, `logs.png`, `ready-url.png`). Recordings stay out of git if large; link them from the README when hosted elsewhere.
