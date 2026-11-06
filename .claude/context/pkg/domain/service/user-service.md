# IUserService (`user-service.go`, `user-view.go`)

## Purpose
Reads and updates the caller's own profile. `buildUserResponse` looks up the space owner and renders the contract's `user` object through `helpers.UserResponse`; auth uses it too.

## Rules
`periodTrackerEnabled` is always the caller's own value. A viewer must never learn the owner's value or gender. The owner fields in `user` are limited to `ownerDisplayName` and `ownerAvatar`.

## Account deletion (`DeleteMe`, `RestoreMe`, `PurgeDue`)
- `DeleteMe` only **schedules**: it sets `deletionRequestedAt` and `deleteAfter` (now + 15 days), revokes every refresh session, and emails the date. Asking again keeps the first date.
- While `deleteAfter` is set the account is locked. `Authenticate` puts it on `bo.Actor.DeleteAfter`, also for a viewer of a pending owner, and `middleware.RequireActiveAccount` answers 403 everywhere except `GET /me`, `DELETE /me` and `POST /me/restore`.
- `RestoreMe` clears both fields before `deleteAfter`; after it, the account counts as deleted.
- `PurgeDue` (`POST /internal/accounts/purge-deleted`, called daily at 10:00 IST by bloom-health-task-worker) and `authService.eraseIfOverdue` (sign-in safety net) call `eraseAccount` (`account-erase.go`): the owner's whole space first, then the user. Space first, so a failure leaves a user who can retry.
- Deleting an owner removes every viewer's access to the space. The viewer accounts remain, without a membership, so they can't sign in to anything until invites exist.
- Account emails (`smtp.AccountEmail`) never fail the request; a send error is only logged.
- Logs `account deletion scheduled`, `account restored`, `accounts purged` (count) and `account erased at sign-in` with ids only.

## Gotchas
- `buildUserResponse` adds two lookups per owner. If they fail, it silently falls back to the caller's own name and avatar.
- A phone is only checked when it actually changes. Saving can still hit a unique-index race, which maps to 409 `already_registered`.

## Related
[me API](../../application/httpserver/v1/public-router/me.md)
