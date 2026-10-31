# pkg/errors/

## Purpose
`IHttpError` and its constructors, plus the typed repository errors `ErrRepoNotFound` and `ErrRepoConflict`.

## Rules
Use only these constructors. Codes are fixed by the contract:

| Code | Status |
|---|---|
| `validation_failed` | 400 |
| `unauthorized`, `token_expired` | 401 |
| `forbidden` | 403 |
| `not_found` | 404 |
| `conflict` | 409, with `details.server` |
| `already_registered` | 409 |
| `rate_limited` | 429, with `Retry-After` |
| `internal` | 500, and 503 from `ServiceUnavailableError` |

## Gotchas
405 is reported with the `validation_failed` code, as the v1 contract requires.
