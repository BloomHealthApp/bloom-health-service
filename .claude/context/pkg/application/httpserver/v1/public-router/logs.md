# Daily logs API (`logs.go`)

Service: [daily_log_service.md](../../../../domain/service/daily_log_service.md). Contract section: Daily logs.

| Route | Access | Success |
|---|---|---|
| `GET /logs?from&to` | both | 200 `{items}` |
| `PUT /logs/:habitId/:localDate` | owner | 200 DailyLog; stale `baseVersion` is 409 with `details.server` |

## Gotchas
- `from` and `to` are both required here, and the range is at most 366 days.
- `GET /logs` includes tombstones, unlike the other list endpoints.
- There is no REST delete for logs; a delete only arrives through `/sync/push`.
- `PUT` to a soft-deleted or unknown habit fails with reason `unknown_habit`. An inactive habit is still accepted.
