# IStatsService (`stats_service.go`)

## Purpose
Computes streaks, completion and workout totals on the server, as a cross-check of the app's own `lib/scoring`.

## Rules
- Use the owner's settings and timezone for every caller, including viewers.
- `periodNeutralDates` passes only dates onward: no flow, symptoms, mood or notes.
- Period days are neutral only while the owner's tracker is enabled.

## Why
`restDay` is the same flag for Sundays and period days, so viewers can't tell them apart.

## Gotchas
- Each request loads all of the space's habits and logs and computes from the earliest log to today. Cost grows with history.
- Sunday rest is on unless `sundayRest` is explicitly false. An unresolved owner falls back to UTC with no period days.
- Only finished workouts count in `workoutCount` and `workoutSeconds`.

## Related
`pkg/utils/scoring/context.md`, `_shared/privacy-invariants.md`
