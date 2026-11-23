# ADR 0004: Single-VM portfolio demo

## Status

Accepted

## Context

Phase 11 needs a controlled public demo without implying multi-tenant commercial hosting.

## Decision

Run API, worker, web, Postgres, Redis, and Traefik on one Linux VM with Docker Compose. Separate `launchbase` and `launchbase-runtime` networks. Restrict sign-in with `AUTH_MODE=github` plus `AUTH_ALLOWLIST`. Cap READY/build concurrency and cron-cleanup old deployments.

## Consequences

Low cost and easy to explain in interviews. Shared Docker host remains a hard security boundary — document limitations honestly and never open anonymous deploys.
