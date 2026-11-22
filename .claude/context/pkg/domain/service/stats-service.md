# IStatsService (`stats-service.go`)

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

## ProfileStats
`ProfileStats(userID)` gives the public-profile numbers for the space the user owns: `currentStreak` and `longestStreak` from the same summary, and the count of active habits. It returns nil (no error) for a viewer, who has no space of their own. Only these three numbers may leave it: never days, `restDay` or anything period-related, because friends see them.

## ProfileActivity
`ProfileActivity(userID)` is a friend's activity graph and today's habits, from the same scoring (`score`) as the summary. Days leave as `scoring.HeatLevel` (mirrors the app's `heatLevel` with the owner's threshold) and only when the level is above 0, so a rest or period day with nothing done is indistinguishable from a missed one. Today's habits are active and scheduled on the owner's local today, ordered by `sortOrder`, with only title, icon and done. Nil for a viewer.
