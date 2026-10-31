# Auth API (`auth.go`)

Service: [auth_service.md](../../../../domain/service/auth_service.md). Contract section: Auth. All routes also pass `LimitAuth` (per IP).

| Route | Success | Per-key limit in the handler |
|---|---|---|
| `POST /auth/signup/request` | 202 `{status, challengeId}` | `signup\|<email>` and `signup\|<phone>`: 429, `Retry-After: 60` |
| `POST /auth/signin/request` | 202 `{status, challengeId}` | `signin\|<identifier>`: still 202 |
| `POST /auth/otp/verify` | 200 tokens + `user` | `verify\|<challengeId>`: 429, `Retry-After: 60` |
| `POST /auth/refresh` | 200 tokens + `user` | none, only `LimitAuth` |
| `POST /auth/logout` | 204 | none, only `LimitAuth` |

## Rules
- Sign-in must never reveal whether an account exists. Over its limit, the handler replaces the identifier with a `decoy|<uuid>` value, so the caller gets the same 202 and nothing is sent.
- Sign-up and sign-in call `Validate()` and then `Normalize()` before the limiter, so limit keys use the canonical email or phone.

## Gotchas
- Sign-up does reveal a taken email or phone (409 `already_registered`). This is an accepted trade-off.
- Logout with an unknown token still returns 204.

## Related
`_shared/security-invariants.md`
