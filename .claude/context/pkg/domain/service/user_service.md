# IUserService (`user_service.go`, `user_view.go`)

## Purpose
Reads and updates the caller's own profile. `buildUserResponse` renders the contract's `user` object and is also used by auth.

## Rules
`periodTrackerEnabled` is always the caller's own value. A viewer must never learn the owner's value or gender. The owner fields in `user` are limited to `ownerDisplayName` and `ownerAvatar`.

## Account deletion (`DeleteMe`)
- Hard delete through `IAccountRepo`: an owner's whole space first (entities of every type including tombstones, applied ops, the sync counter, all memberships), then the user (chats, refresh sessions, pending codes for their email, memberships, the user document). Space first, so a failure leaves a user who can retry.
- Deleting an owner removes every viewer's access to the space. The viewer accounts remain, without a membership, so they can't sign in to anything until invites exist.
- Access tokens stop working at once because `Authenticate` loads the user from the database on every request.
- Logs `account erased` with the user id only, as a record that a deletion request was carried out.

## Gotchas
- `buildUserResponse` adds two lookups per owner. If they fail, it silently falls back to the caller's own name and avatar.
- A phone is only checked when it actually changes. Saving can still hit a unique-index race, which maps to 409 `already_registered`.

## Related
[me API](../../application/httpserver/v1/public-router/me.md)
