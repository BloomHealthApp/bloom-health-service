# IDailyLogService (`daily_log_service.go`)

## Purpose
Lists logs and writes one per habit and date. The id is `LogID(habitId, localDate)`, so it's `habitId:localDate`.

## Rules
Writes go straight through `ISyncService.Apply`, not `entityWriter`, because `PUT` creates or updates and uses `baseVersion`.

## Gotchas
- `Put` sends `value` and `note` as null when omitted, so leaving them out clears them.
- If `status` is omitted, `Apply` computes it from the value and the habit's target.
- `List` includes tombstones.
