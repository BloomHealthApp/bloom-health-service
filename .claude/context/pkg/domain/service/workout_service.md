# IWorkoutService (`workout_service.go`)

## Purpose
Lists workouts and wraps writes through [entity_writer.md](entity_writer.md). Deleting writes a tombstone.

## Related
Field rules such as `endedAt >= startedAt` are in `pkg/infrastructure/validation/`.
