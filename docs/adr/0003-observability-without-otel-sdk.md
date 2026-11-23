# ADR 0003: Observability without the OpenTelemetry SDK

## Status

Accepted

## Context

Phase 10 needs structured logs, metrics, and a trace story. Pulling the full OTel Go SDK adds weight and exporter configuration for a single-host portfolio platform.

## Decision

Use stdlib JSON line logs, a small in-process Prometheus text exposition on `/metrics`, W3C `traceparent` parse/generate hooks, and document how to extend job payloads with `trace_id` later. Optional Prometheus/Grafana compose profile scrapes host metrics endpoints.

## Consequences

Fast local diagnosis with `curl` and log aggregation. Not a full distributed tracing backend out of the box. Adding OTLP later remains compatible with the same `traceparent` headers.
