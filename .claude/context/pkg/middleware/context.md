# pkg/middleware/

## Purpose
Global chain, in the order set by `pkg/application/httpserver/router.go`: generic request info, recovery, secure headers, CORS, request logger, per-IP rate limit, body limit. Auth, role and `/auth` rate limit middleware are applied per route. The `/v1` group also gets `RequestTimeout(constants.RequestTimeout)` (12s, so a 100-operation sync push fits): it puts a deadline on the request context so a stuck database call ends as a 503 instead of hanging until the host kills it. Keep it under the app's 15s client timeout. `/internal/*` (the purge job) is outside `/v1` and has no deadline.

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
