# Period tracker API (`periods.go`)

Service: [period_service.md](../../../../domain/service/period_service.md). Contract section: Period tracker.

| Route | Access | Success |
|---|---|---|
| `GET /periods?from&to` | owner, tracker on | 200 `{items}` ascending by `startDate` |
| `POST /periods` | owner, tracker on | 201 Period |
| `PATCH /periods/:id` | owner, tracker on | 200 Period; `version` required, stale is 409 |
| `DELETE /periods/:id` | owner, tracker on | 204, tombstone |

## Rules
This is private health data, and `_shared/privacy-invariants.md` always applies. Even the read route sits in the owner group, unlike every other `GET`.

## Gotchas
- `from` and `to` select periods that overlap the range, unlike the `localDate` filter on logs and workouts. An ongoing period has no end, so it matches any `from`.
- Viewers and disabled trackers both get 403.
