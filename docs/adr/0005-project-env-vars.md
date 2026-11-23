# ADR 0005: Project environment variables

## Status

Accepted (Phase 12)

## Context

Deployed apps (e.g. Vectra) need API keys. The MVP passed no `-e` into user containers. We need encrypted at-rest storage, masked API responses, log redaction, and injection at container start — without giving apps LaunchBase infrastructure secrets.

## Decision

1. Store per-project env rows in Postgres (`project_env_vars`) with AES-GCM ciphertext.
2. Key material from `ENV_ENCRYPTION_KEY` (preferred) or `SESSION_SECRET`.
3. API lists only key names + masked hints; create/update/delete require project ownership.
4. Worker decrypts at start time and passes `-e KEY=value` into `docker run`.
5. Block reserved keys (`DATABASE_URL`, `SESSION_SECRET`, `LAUNCHBASE_*`, …).
6. Append decrypted values to log redaction for that deployment only.
7. Single environment label `production` for now (column reserved for later staging).

## Consequences

Redeploy (or new deploy) required to pick up env changes on a running container. Operators must set encryption key in production. Apps can receive secrets; LaunchBase secrets stay blocked by docker arg validation.
