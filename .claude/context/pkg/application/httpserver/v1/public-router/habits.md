# Habits API (`habits.go`)

Service: [habit_service.md](../../../../domain/service/habit_service.md). Contract section: Habits.

| Route | Access | Success |
|---|---|---|
| `GET /habits?includeInactive=bool` | both | 200 `{items}` sorted by `sortOrder` |
| `POST /habits` | owner | 201 Habit |
| `PATCH /habits/:id` | owner | 200 Habit; stale `version` is 409 with `details.server` |
| `DELETE /habits/:id` | owner | 204 |

## Rules
`bindCreate` and `bindPatch` in this file are shared with workouts, periods and settings. Changing them changes all of those APIs.

## Gotchas
- `DELETE` deactivates (`isActive: false`) and doesn't write a tombstone. Restore with `PATCH isActive: true`.
- Only the literal `includeInactive=true` counts; any other value means false.
