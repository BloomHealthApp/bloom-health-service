# IChatService (`chat-service.go`)

## Purpose
The AI chat ("Bloomy") API. It stores turns and enqueues a pending assistant message; it never calls a model. `bloom-health-task-worker` (separate repo) claims, answers and settles those messages. Shared shapes and lifecycle: `docs/chat-contract.md`.

## Rules
- Every lookup is scoped to `actor.User.ID`; another user's session is 404, never 403.
- One turn in flight per session: `IChatSessionRepo.BeginTurn` is a compare-and-swap on `activeTurnId == null`. There are no transactions, so if inserting the two messages fails, `SendMessage` releases the turn with `EndTurn` before returning 500.
- The assistant message `_id` is the session's `activeTurnId`; the worker clears it with a compare-and-swap on that id.
- A chat whose `modelId` has left the catalogue (a retired model) sends new messages with the default model instead.
- Model ids are validated against `config.ChatConfig` (`WithDefaults` gives tests and a missing config section the shipped catalogue). Unknown ids are 400.
- `contextScopes` is stored as `[]` and non-empty values are rejected in the DTO until the worker has providers for them.

## Why
- Mongo as the queue (instead of Kafka/Redis) keeps the service's no-extra-infrastructure model; the worker's claim index lives in this service's `EnsureIndexes` because this service owns the collections.
- The first message replaces the default title so the session list is readable; the title is user text and is redacted from logs.

## Related
`_shared/privacy-invariants.md` (chat text), `public-router/chat.md`

## Deleting a chat
`DeleteSession` soft-deletes the session (clearing its title, which is chat text) and then hard-deletes all its messages. If the worker is mid-turn, its settle matches nothing and it treats the turn as settled elsewhere. MongoDB's TTL index removes the tombstone after 7 days.
