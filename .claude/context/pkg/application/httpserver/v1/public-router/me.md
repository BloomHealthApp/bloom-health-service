# Profile API (`get-me.go`, `patch-me.go`, `delete-me.go`, `request-phone-change.go`, `verify-phone-change.go`)

Service: [user-service.md](../../../../domain/service/user-service.md). Contract section: Profile & settings.

| Route | Access | Success |
|---|---|---|
| `GET /me` | both | 200 `user` |
| `PATCH /me` | both | 200 `user` |
| `POST /me/phone/request` | both | 202 `{status, challengeId}`; code mailed to the caller's email |
| `POST /me/phone/verify` | both | 200 `user`; saves the phone |
| `DELETE /me` | both | 202 `{deleteAfter}`; locks the account, erased after 15 days |
| `POST /me/restore` | both | 200 `user`; cancels a pending deletion |

## Rules
- A caller only ever changes their own profile, so `PATCH /me` is deliberately open to viewers.
- `request.OptString` / `OptBool` tell an absent field from an explicit `null`. `null` clears `avatar` and `gender`, and resets `periodTrackerEnabled` to the gender default.
- `PATCH /me` can't set a phone. A phone is only saved after `POST /me/phone/verify`, so the caller has proved they control the account's email. This doesn't prove they own the number; only an SMS code could.
- A wrong code on `/me/phone/verify` is a 400, not a 401, because a 401 makes clients treat the session as expired.

- `DELETE /me` is what Google Play's account-deletion rule needs. It takes no body and no code: the app asks for confirmation, and a signed-in caller can only schedule their own account. It doesn't erase anything itself: the account is locked for 15 days (`constants.AccountDeletionGrace`) and then erased by the daily purge (bloom-health-task-worker calls `POST /internal/accounts/purge-deleted` at 10:00 IST) or at its next sign-in.
- `GET /me`, `DELETE /me` and `POST /me/restore` are registered on the `signedIn` group, outside `RequireActiveAccount`, so a locked account can still see its deletion date and restore. Every other authenticated route goes through `RequireActiveAccount` and answers 403 `account_pending_deletion`.

## Gotchas
A taken phone returns 409 `already_registered` from either phone route, whether found by lookup or by a unique-index race on save.
