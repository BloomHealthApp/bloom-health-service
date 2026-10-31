# Stats API (`stats.go`)

Service: [stats_service.md](../../../../domain/service/stats_service.md). Contract section: Stats.

| Route | Access | Success |
|---|---|---|
| `GET /stats/summary?from&to` | both | 200 summary |

## Rules
- Viewers get the owner's numbers.
- `restDay` must never say why a day is neutral, because that would expose period days.

## Gotchas
- `to` defaults to the owner's today, and `from` to 29 days before `to`.
- The range only limits `days` and workout totals. Streaks and milestones are always computed over all history.
