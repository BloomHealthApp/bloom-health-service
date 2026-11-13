# Chat queue contract (bloom-health-service ⇄ bloom-health-task-worker)

The AI chat ("Bloomy") is split across two services that share one MongoDB database:

- **bloom-health-service** owns the REST API (`docs/api-contract.md`, "Chat"), creates sessions, stores user messages and enqueues pending assistant messages. It never calls a model and holds no model API key.
- **bloom-health-task-worker** claims pending assistant messages, calls the model, and settles them.

Both repos keep their own copy of this file and of the document shapes below. A change to a field or rule must land in both.

## Collections

### `chatSessions`
| Field | Type | Written by | Notes |
|---|---|---|---|
| `_id` | string (UUID) | service | |
| `userId` | string | service | the only user who may read or write the session |
| `spaceId` | string | service | the user's space, for future context providers |
| `title` | string | service | private user text: never log |
| `modelId` | string | service | catalogue id (below), default for new messages |
| `contextScopes` | string[] | service | always `[]` in v1; future: `habitStreaks`, `healthReport` |
| `activeTurnId` | string \| null | service sets, both clear | `_id` of the in-flight assistant message |
| `nextIdx` | int | service | next free message `idx`; each turn reserves two |
| `version` | int | both | `$inc` on every update |
| `createdAt`, `updatedAt` | date | service | |
| `deletedAt` | date \| null | service | set on delete, when the service also clears `title` and erases the session's messages; a TTL index removes the session 7 days later |

### `chatMessages`
| Field | Type | Written by | Notes |
|---|---|---|---|
| `_id` | string (UUID) | service | assistant `_id` == the session's `activeTurnId` |
| `sessionId`, `userId` | string | service | |
| `idx` | int | service | unique per session, 0-based |
| `role` | `user` \| `assistant` | service | |
| `content` | string | service (user), worker (assistant) | private: never log |
| `status` | `pending` \| `running` \| `succeeded` \| `failed` | both | user messages are stored `succeeded` |
| `modelId` | string | service | catalogue id to answer with |
| `attempt` | int | worker | `$inc` on each claim |
| `runAt` | date | service, worker | not claimable before this time (retry backoff) |
| `lockedUntil` | date \| null | worker | lease of the running attempt |
| `errorCode` | string | worker | set when `failed` |
| `usage` | `{inputTokens, outputTokens}` | worker | |
| `finishReason`, `latencyMs` | string, int | worker | |
| `traceparent` | string, optional | service | W3C trace context of the request that queued the turn; the worker continues that trace. Absent on older rows |
| `createdAt`, `updatedAt` | date | both | |

Clients see `running` as `pending`.

## Indexes
Created by the service at startup (`EnsureIndexes`) and documented in `ddl/mongodb/indexes.js`:
- `chatSessions {userId: 1, updatedAt: -1}`
- `chatMessages {sessionId: 1, idx: 1}` unique
- `chatMessages {status: 1, runAt: 1, createdAt: 1}`: the worker's claim query
- `chatMessages {userId: 1, role: 1, createdAt: 1}`: the service's daily cap

## Lifecycle
1. **Enqueue (service).** Compare-and-swap on the session: `{_id, userId, deletedAt: null, activeTurnId: null}` → set `activeTurnId = <new assistant id>`, `$inc nextIdx 2`. If no document matches, the session is missing (404) or a turn is in flight (409). Then insert the user message (`succeeded`) and the assistant message (`pending`, `attempt 0`, `runAt = now`). If the insert fails, clear `activeTurnId` (compare-and-swap on the same id).
2. **Claim (worker).** `findOneAndUpdate({status: "pending", runAt: {$lte: now}}, sort {createdAt: 1})` → `status: running`, `$inc attempt`, `lockedUntil = now + timeout`. If the session is deleted, fail the message with `session_deleted`. Deleting a session erases its messages, so a turn in flight may find its message gone: the settle then matches nothing and the worker drops the result.
3. **Settle (worker).** Every update filters `{_id, status: "running", attempt: <claimed attempt>}` so a stale worker cannot overwrite a newer attempt:
   - success → `succeeded`, `content`, `usage`, `finishReason`, `latencyMs`;
   - retryable error (network, 429, 5xx) with attempts left → `pending`, `runAt = now + backoff`;
   - otherwise → `failed` with `errorCode`.
   After a final status, clear the session's `activeTurnId` only if it still equals this message's `_id`.
4. **Reap (worker).** `running` past `lockedUntil` → `failed` (`worker_lost`); `pending` older than 30 minutes → `failed` (`expired`); clear `activeTurnId` for each.

## Model catalogue
Config, not a collection, in both repos (`chat.models[]`). The service validates ids and lists `id`, `label`, `vendor`, `default`; the worker routes on `vendor` and sends `providerModelId`.

| id | vendor | providerModelId | contextWindow |
|---|---|---|---|
| `groq-gpt-oss-120b` (default) | `groq` | `openai/gpt-oss-120b` | 131072 |

## Privacy
- Message content and titles are private user data: neither service logs them (both loggers redact `content`, `prompt`, `title`).
- v1 sends only the conversation and a fixed system prompt to the model. Future context providers are selected by `contextScopes`; period and gender data are never included unless a scope explicitly allows it for the owner and the user has opted in.
