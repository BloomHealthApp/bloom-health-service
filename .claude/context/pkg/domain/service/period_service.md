# IPeriodService (`period_service.go`)

## Purpose
The private period tracker. Writes go through [entity_writer.md](entity_writer.md), and deleting writes a tombstone.

## Rules
- Every method calls `guard` first: owner only, and tracker enabled.
- Don't add a method that skips `guard`, or a path that returns period rows to a viewer.
- Disabling the tracker keeps the data.

## Gotchas
- `List` filters overlap in Go after loading all of the space's non-deleted periods, because the repo's range filter only works on `localDate`.
- `Apply` checks the tracker again, because `/sync/push` doesn't pass through this service.
- Overlap and one-ongoing rules are in `syncService.checkPeriodRules`.

## Related
`_shared/privacy-invariants.md`
