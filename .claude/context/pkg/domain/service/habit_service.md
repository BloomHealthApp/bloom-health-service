# IHabitService (`habit_service.go`)

## Purpose
Lists habits and wraps writes through [entity_writer.md](entity_writer.md).

## Rules
Deleting deactivates with `isActive: false` and keeps the logs.

## Gotchas
A `delete` op through `/sync/push` does write a tombstone, unlike REST `DELETE`. Later logs for that habit are rejected with `unknown_habit`.
