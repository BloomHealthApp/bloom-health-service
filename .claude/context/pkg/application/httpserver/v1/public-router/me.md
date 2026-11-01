# Profile API (`me.go`)

Service: [user_service.md](../../../../domain/service/user_service.md). Contract section: Profile & settings.

| Route | Access | Success |
|---|---|---|
| `GET /me` | both | 200 `user` |
| `PATCH /me` | both | 200 `user` |
| `POST /me/phone/request` | both | 202 `{status, challengeId}`; code mailed to the caller's email |
| `POST /me/phone/verify` | both | 200 `user`; saves the phone |

## Rules
- A caller only ever changes their own profile, so `PATCH /me` is deliberately open to viewers.
- `request.OptString` / `OptBool` tell an absent field from an explicit `null`. `null` clears `avatar` and `gender`, and resets `periodTrackerEnabled` to the gender default.
- `PATCH /me` can't set a phone. A phone is only saved after `POST /me/phone/verify`, so the caller has proved they control the account's email. This doesn't prove they own the number; only an SMS code could.
- A wrong code on `/me/phone/verify` is a 400, not a 401, because a 401 makes clients treat the session as expired.

## Gotchas
A taken phone returns 409 `already_registered` from either phone route, whether found by lookup or by a unique-index race on save.
