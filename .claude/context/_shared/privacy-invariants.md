# Period tracker and neutral days: privacy invariants (do not weaken)

- Period entries (entityType `period`, collection `periods`, REST `/periods*`) are **owner-only health data**: viewers get 403 on `/periods*`, `ISyncService.Pull` filters `period` rows out for non-owners (`changes.periods` is `[]`, cursor still advances), viewer pushes are rejected, and no other endpoint returns period rows. Do not add a code path that reads `entitytype.Period` for a viewer; stats only expose the generic `restDay` flag.
- The tracker is on when `User.IsPeriodTrackerEnabled()`: the explicit `periodTracker` setting if set, else `gender == female`. When off, `/periods*` and period ops (`ISyncService.Apply`) are refused (403 / rejected `period_tracker_disabled`) and period days stop being neutral; data is kept.
- Cross-record rules (no overlap, one ongoing) live in `syncService.checkPeriodRules`, because they need sibling rows; field rules are in `validation.validatePeriod`.
- Streak neutrality is computed in `statsService`: `RestDays = settings.restDays + {0 when sundayRest true/absent}`, `NeutralDates` = owner's non-deleted periods (ongoing = through today) only if the owner's tracker is enabled. The scoring library stays pure.
- Logs must never contain period/gender data (logger redacts `period`, `gender`, `symptom`, `mood`, `flow`, `startdate`, `enddate` keys). Never log entity `Data`.
