# pkg/utils/scoring/

## Purpose
Streak and completion rules. Fixtures are in `testdata/scoring.json`.

## Rules
- The library stays pure. `Settings.NeutralDates` behave like rest days, and callers (`statsService`) compute that set.
- The same rules are mirrored in the mobile app, so a rule change here needs the app and the fixtures updated too.

## Related
`_shared/privacy-invariants.md`
