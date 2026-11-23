# ADR 0001: API never controls Docker

## Status

Accepted

## Context

LaunchBase must accept untrusted repositories and produce running containers. Putting Docker in the HTTP process couples request latency to builds and expands the blast radius of a compromised API.

## Decision

The API persists projects/deployments and publishes jobs through a Postgres outbox to Redis. Only the worker process talks to the Docker daemon.

## Consequences

Horizontal scale of HTTP does not multiply Docker load. Deployments survive API restarts. Operators must run two processes. Debugging spans API logs, queue depth, and worker logs.
