# ADR 0002: Postgres outbox + Redis stream

## Status

Accepted

## Context

Creating a deployment must be durable even when Redis is briefly down. At-least-once delivery is acceptable if workers are idempotent.

## Decision

`POST .../deployments` writes the deployment and an outbox row in one transaction. A publisher loop (API and worker) pushes unpublished rows to the Redis stream. Workers lock by deployment ID and treat duplicate deliveries as the same job.

## Consequences

No dual-write loss when Redis is down. Duplicate processing is possible after publish/commit races; container names stay stable so retries replace rather than stack. Operators inspect `deployment_outbox` and `launchbase_queue_depth` when jobs stall.
