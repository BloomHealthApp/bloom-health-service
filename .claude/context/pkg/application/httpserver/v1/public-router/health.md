# Health API (`health.go`)

Service: [health_service.md](../../../../domain/service/health_service.md). Contract section: Health.

| Route | Access | Success |
|---|---|---|
| `GET /health`, `GET /v1/health` | public | 200 `{status, time, version}` |

## Rules
Never expose database details here.

## Gotchas
- A failed ping returns 503 with the `internal` code, which the contract doesn't list.
- `/health` is registered in `pkg/application/httpserver/router.go`, and `/v1/health` in `public-router/router.go`.
- The global per-IP rate limit still applies, so frequent uptime probes from one IP count against it.
