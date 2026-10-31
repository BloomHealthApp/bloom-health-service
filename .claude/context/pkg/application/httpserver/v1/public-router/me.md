# Profile API (`me.go`)

Service: [user_service.md](../../../../domain/service/user_service.md). Contract section: Profile & settings.

| Route | Access | Success |
|---|---|---|
| `GET /me` | both | 200 `user` |
| `PATCH /me` | both | 200 `user` |

## Rules
- A caller only ever changes their own profile, so `PATCH /me` is deliberately open to viewers.
- `request.OptString` / `OptBool` tell an absent field from an explicit `null`. `null` clears `avatar` and `gender`, and resets `periodTrackerEnabled` to the gender default.
- `Normalize()` rewrites `phone` to E.164 before the service runs.

## Gotchas
A taken phone returns 409 `already_registered`, whether found by lookup or by a unique-index race on save.
