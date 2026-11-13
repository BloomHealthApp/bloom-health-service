# pkg/telemetry/

## Purpose
OpenTelemetry setup (`Setup`, called from `cmd/server/main.go`), the service's metric instruments, and a MongoDB command monitor (`MongoMonitor`, attached in `pkg/drivers/mongodb.go`).

## Rules
- Export is on only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (`config.Store.OTLPEndpoint`); the exporters read `OTEL_EXPORTER_OTLP_HEADERS` (secret) and other `OTEL_*` variables themselves. Off, the W3C propagator is still installed, so traceparents still pass through to logs and chat turns.
- **Privacy:** attributes are routes, methods, statuses, entity types, ops, sync results, model ids, collection and command names. Never paths, queries, bodies, user ids, MongoDB command documents or server error messages (a duplicate-key error quotes the key, e.g. an email); Mongo failures are recorded by error code only.
- Instruments are created lazily from the global meter, which binds to the first provider installed. Tests that need metrics install a provider once per process (`telemetry_test.go` in httpserver).
- Short export intervals (2 s span batches, 15 s metrics) because a Vercel instance can be frozen when idle; shutdown flushes.

## Metrics
`http.server.request.duration`, `db.client.operation.duration`, `bloom.sync.operations` (entity_type, op, result), `bloom.chat.turns.enqueued` (model).

## Related
Queued chat turns store `traceparent` (`docs/chat-contract.md`) so bloom-health-task-worker continues the trace.
