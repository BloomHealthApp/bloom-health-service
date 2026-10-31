# pkg/middleware/

## Purpose
Global chain, in the order set by `pkg/application/httpserver/router.go`: generic request info, recovery, secure headers, CORS, request logger, per-IP rate limit, body limit. Auth, role and `/auth` rate limit middleware are applied per route.

## Rate limits
Set in `ratelimit.NewSet`.

| Limiter | Key | Rate | Burst |
|---|---|---|---|
| `IP` | client IP, every request | 30 per second | 90 |
| `Auth` | client IP, `/v1/auth/*` | 1 per 6 seconds | 10 |
| `Email` | email, phone or challenge, in auth handlers | 1 per 2 minutes | 3 |

## Why
Per-email rate limits live in the handlers, not here, because they depend on the request body.

## Related
`_shared/security-invariants.md`
