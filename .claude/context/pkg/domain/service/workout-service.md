# IWorkoutService (`workout-service.go`)

## Purpose
Lists workouts and wraps writes through [entity-writer.md](entity-writer.md). Deleting writes a tombstone.

## Related
Field rules such as `endedAt >= startedAt` are in `pkg/infrastructure/validation/`.
