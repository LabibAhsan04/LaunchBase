# Observability

## Signals

| Signal | Where | Notes |
| --- | --- | --- |
| Structured JSON logs | API and worker stdout | One object per line: `ts`, `level`, `service`, `msg`, plus fields |
| Prometheus text metrics | `GET /metrics` on API `:8080` and worker `:8081` | No third-party Prometheus client library |
| Liveness | `GET /health` | Process is up (worker may report `draining`) |
| Readiness | `GET /ready` | API: Postgres + Redis. Worker: same, fails while draining. Bodies never include dependency errors |

## Log fields

Common fields when present:

- `requestId`, `traceId`, `spanId` (HTTP)
- `deploymentId`, `projectId`, `attempt`, `outcome`, `durationMs` (worker jobs)
- `method`, `path`, `status` (HTTP access)

Secrets must never appear in these fields. Build log redaction is unchanged (`docs/security.md`).

## Metrics

| Metric | Meaning |
| --- | --- |
| `launchbase_http_requests_total` | HTTP count by method, path, status |
| `launchbase_http_request_duration_ms_*` | HTTP latency sum/count |
| `launchbase_queue_depth` | Unpublished outbox rows |
| `launchbase_job_duration_ms_*` | Worker job latency by outcome |
| `launchbase_deployments_total` | Deployment outcomes (`ok`, `failed`, `busy`, `error`) |
| `launchbase_active_containers` | READY deployment count (worker sample) |
| `launchbase_build_failures_total` | Failed job count |

UUID path segments are replaced with `:id` before labeling.

## Trace context (OpenTelemetry-compatible, no SDK)

1. Inbound HTTP accepts W3C `traceparent` when present.
2. Otherwise the API generates a 16-byte trace ID and 8-byte span ID.
3. Responses expose `X-Request-ID` and `traceparent`.
4. Redis job payloads do not yet carry `trace_id`. Correlate worker lines with `deploymentId` / `projectId`. Adding `trace_id` to `queue.Job` is the documented next step for cross-process traces.
5. OTLP exporters are omitted to keep dependencies small. Local diagnosis uses JSON logs and `/metrics`.

## Optional Prometheus / Grafana

```bash
docker compose -f docker-compose.yml -f docker-compose.observability.yml --profile observability up -d
```

- Prometheus: http://localhost:9090 (scrapes `host.docker.internal:8080` and `:8081`)
- Grafana: http://localhost:3001 (admin / admin)

Leave this profile off for normal development.

## Diagnosing a failed deployment

1. Open the deployment page or `GET /api/v1/deployments/{id}` — note `status` and `error`.
2. Pull build lines: `GET /api/v1/deployments/{id}/logs` (or the live SSE stream).
3. In worker stdout, find `job_finished` with that `deploymentId` and `outcome=failed` plus `durationMs`.
4. Confirm `launchbase_build_failures_total` and `launchbase_deployments_total{outcome="failed"}` moved on the worker `/metrics`.
5. If the job never started, check API `launchbase_queue_depth` and `/ready` on both processes.
