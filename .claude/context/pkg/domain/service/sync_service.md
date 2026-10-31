# ISyncService (`sync_service.go`)

## Purpose
The single write path for every entity (`Apply`), plus the offline-sync batch endpoints (`Push`, `Pull`).

## Apply order
1. Reject non-owners (`forbidden`), then period writes while the tracker is off (`period_tracker_disabled`).
2. If `operationId` is set and already stored, replay the stored result. `applied` becomes `duplicate`.
3. Check entity type, op and id. Daily logs must have an id of `habitId:localDate`, and the habit must exist.
4. Compare `baseVersion` with the stored version. See conflicts below.
5. Compute the log status if it's missing, using `scoring.EvaluateStatus`.
6. `validation.Merge`, then `checkPeriodRules` for periods.
7. Save with compare-and-swap, then store the op record.

## Rules
- Only infrastructure failures return an error. Conflicts and rejections are results.
- Conflicts: the server record wins, except for a daily log upsert whose `clientTimestamp` is newer than `updatedAt`. That one applies with `resolved: "lww"`.
- Rejected and conflict results are stored as op records too, so a retry gets the same answer.

## Gotchas
- REST writes have no `operationId`, so they have no idempotency record.
- An `operationId` already used by another space is rejected with `operation_id_in_use`.
- Deleting a record that's already deleted returns `applied`, even with a stale version.
- Op records expire after 60 days (`OpRetention`). The contract promises at least 30.
- Upserting a tombstoned entity brings it back, because `DeletedAt` is cleared.
- "Today" in validation uses the actor's timezone.
- `Pull` fetches `limit+1` rows to set `hasMore`, and advances the cursor past rows hidden from viewers.

## Related
[entity_writer.md](entity_writer.md), `_shared/privacy-invariants.md`, [sync API](../../application/httpserver/v1/public-router/sync.md)
