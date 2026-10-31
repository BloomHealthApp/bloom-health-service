# entityWriter and shared helpers (`entity_writer.go`)

## Purpose
Create, patch and delete for one entity type on top of `ISyncService.Apply`. Used by the habit, workout and period services.

- `requireOwner`: service-level viewer guard, behind the `RequireOwner` middleware.
- `resultToEntity`: maps an `ApplyResult` to the REST response.

| Result | HTTP |
|---|---|
| applied, duplicate | record |
| conflict | 409 with `details.server` |
| rejected `forbidden`, `period_tracker_disabled` | 403 |
| rejected `not_found` | 404 |
| any other rejection | 400 |

## Rules
`find` runs first on PATCH and DELETE so they never create records, and return 404 for ids that exist only in another space.

## Gotchas
- `remove` uses the current version, so a REST delete can't conflict on a stale client version, unlike PATCH.
- A new rejection reason becomes a 400 unless `resultToEntity` is updated.
