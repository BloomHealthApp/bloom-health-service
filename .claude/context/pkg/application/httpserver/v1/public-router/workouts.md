# Workouts API (`workouts.go`)

Service: [workout_service.md](../../../../domain/service/workout_service.md). Contract section: Workouts.

| Route | Access | Success |
|---|---|---|
| `GET /workouts?from&to` | both | 200 `{items}`, non-deleted |
| `POST /workouts` | owner | 201 Workout |
| `PATCH /workouts/:id` | owner | 200 Workout; `version` required, stale is 409 |
| `DELETE /workouts/:id` | owner | 204, tombstone |

## Gotchas
`from` and `to` are optional and filter on `data.localDate`.
