# pkg/infrastructure/mongorepo/

## Purpose
Repository interfaces and MongoDB implementations, with in-memory fakes in `memory/` and `mock_gen.go`.

## Rules
- One interface per aggregate, in the same `<domain>-repo.go` file as its MongoDB implementation; the matching fake is `memory/<domain>.go`.
- Constructor shape: `NewXxxRepo(db drivers.IMongoDB, log)`.
- Return `apperrors.ErrRepoNotFound` / `ErrRepoConflict`.
- Mirror every behavior change in `memory/`, which backs the integration tests.

## Related
`_shared/testing-limits.md`, `ddl/mongodb/context.md`

## Gotchas
- `chat-session-repo.go` and `chat-message-repo.go` shares its collections with `bloom-health-task-worker`; field names are fixed by `docs/chat-contract.md`. `BeginTurn` returns the session's `nextIdx` before the `$inc` and tells "missing" from "turn in flight" with a follow-up `Get`.
- `memory.SettleForTest` and `memory.ChatSessionTombstoneForTest` let HTTP tests play the worker's part or inspect a deleted session; they are not part of the interfaces.
- `connection-repo.go` (`IConnectionRepo`): one document per pair of users (`pairKey`, unique, both directions). `ListForUser` leaves out `declined` cooldown markers, which a TTL index on `expiresAt` removes. `users.profileLinkCode` has a partial unique index (only string values), like `googleSub`.
- `direct-message-repo.go` (`IDirectMessageRepo`): `dmThreads` (one per friend pair, `_id` = pair key, `nextSeq` counter, `last` preview, `readAt` per member) and `directMessages` (`_id` = the app's UUID, unique `{pairKey, seq}`). `SetLast` only moves forward; `MarkRead` uses `$max`. `push-token-repo.go` (`IPushTokenRepo`): `pushTokens`, `_id` = the Expo token, so a device belongs to one user.
- `account-repo.go` (`IAccountRepo`) erases across collections for `DELETE /v1/me`. Any new collection holding user or space data must be added (connections, chats with friends and push tokens are, in `EraseUser`; messages are found through the user's threads, so they are deleted before the threads) to `EraseSpace` or `EraseUser` (and to `memory/account.go`), or account deletion leaves it behind.
- Deleting a chat hard-deletes its messages and keeps a title-less tombstone; a TTL index on `chatSessions.deletedAt` removes it after `constants.DeletedChatRetention` (7 days).
