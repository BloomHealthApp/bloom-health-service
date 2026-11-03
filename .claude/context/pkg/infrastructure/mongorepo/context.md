# pkg/infrastructure/mongorepo/

## Purpose
Repository interfaces and MongoDB implementations, with in-memory fakes in `memory/` and `mock_gen.go`.

## Rules
- One interface per aggregate in `interfaces.go`.
- Constructor shape: `NewXxxRepo(db drivers.IMongoDB, log)`.
- Return `apperrors.ErrRepoNotFound` / `ErrRepoConflict`.
- Mirror every behavior change in `memory/`, which backs the integration tests.

## Related
`_shared/testing-limits.md`, `ddl/mongodb/context.md`

## Gotchas
- `chat.go` (chat sessions and messages) shares its collections with `bloom-health-task-worker`; field names are fixed by `docs/chat-contract.md`. `BeginTurn` returns the session's `nextIdx` before the `$inc` and tells "missing" from "turn in flight" with a follow-up `Get`.
- `memory.SettleForTest` and `memory.ChatSessionTombstoneForTest` let HTTP tests play the worker's part or inspect a deleted session; they are not part of the interfaces.
- `account.go` (`IAccountRepo`) erases across collections for `DELETE /v1/me`. Any new collection holding user or space data must be added to `EraseSpace` or `EraseUser` (and to `memory/account.go`), or account deletion leaves it behind.
- Deleting a chat hard-deletes its messages and keeps a title-less tombstone; a TTL index on `chatSessions.deletedAt` removes it after `constants.DeletedChatRetention` (7 days).
