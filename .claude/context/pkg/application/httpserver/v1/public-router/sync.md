# Sync API (`sync.go`)

Service: [sync_service.md](../../../../domain/service/sync_service.md). Contract section: Sync.

| Route | Access | Success |
|---|---|---|
| `POST /sync/push` | owner | 200 `{results}`, one per operation, in order |
| `GET /sync/pull?cursor&limit` | both | 200 `{cursor, hasMore, changes}` |

## Rules
- A push returns 200 even when individual operations conflict or are rejected. Read `results[].status`.
- Viewers must never receive `period` rows from a pull, but their cursor still advances past them.

## Gotchas
- A push must have 1-100 operations, or it's a 400.
- An `operationId` that is empty or longer than 64 characters gets a `rejected` result, and the rest of the batch continues.
- An infrastructure failure returns 500 for the whole request, even though earlier operations are already applied. Retrying is safe because of `operationId`.
- A pull `limit` of 0 or over 1000 silently becomes the default 500; it isn't capped at 1000. A non-numeric limit is a 400.
