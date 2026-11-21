# Health API (`get-health.go`)

Service: [health-service.md](../../../../domain/service/health-service.md). Contract section: Health.

| Route | Access | Success |
|---|---|---|
| `GET /health`, `GET /v1/health` | public | 200 `{status, time, version}` |
| `GET /livez` (`get-livez.go`) | public | 200 `{status: "ok"}` |

## Rules
Never expose database details here.

`/livez` must stay dependency-free: it is the Kubernetes liveness probe, and checking MongoDB there would restart every pod during a database outage. `/health` (which pings MongoDB) is the readiness probe. `/livez` is registered only in `pkg/application/httpserver/router.go`, outside `/v1`.

## Gotchas
- A failed ping returns 503 with the `internal` code, which the contract doesn't list.
- `/health` is registered in `pkg/application/httpserver/router.go`, and `/v1/health` in `public-router/router.go`.
- The global per-IP rate limit still applies, so frequent uptime probes from one IP count against it.
