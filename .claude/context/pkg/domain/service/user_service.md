# IUserService (`user_service.go`, `user_view.go`)

## Purpose
Reads and updates the caller's own profile. `buildUserResponse` renders the contract's `user` object and is also used by auth.

## Rules
`periodTrackerEnabled` is always the caller's own value. A viewer must never learn the owner's value or gender. The owner fields in `user` are limited to `ownerDisplayName` and `ownerAvatar`.

## Gotchas
- `buildUserResponse` adds two lookups per owner. If they fail, it silently falls back to the caller's own name and avatar.
- A phone is only checked when it actually changes. Saving can still hit a unique-index race, which maps to 409 `already_registered`.

## Related
[me API](../../application/httpserver/v1/public-router/me.md)
