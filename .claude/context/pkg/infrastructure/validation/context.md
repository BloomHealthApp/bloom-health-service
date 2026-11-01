# pkg/infrastructure/validation/

## Purpose
Strict allowlist validation for entities: habit, dailyLog, workout, settings, period.

## Rules
Field rules live here. Period rules that need sibling rows live in `syncService.checkPeriodRules`.

Daily log `source` is `manual` or `device`. `Merge` sets `manual` when the payload omits it, unlike other fields, which keep their stored value.

## Why
- The allowlists also prevent mass-assignment of server-controlled fields.
- Older apps never send `source`, so an edit from one must not keep a stale `device` value. The app relies on `source` to avoid overwriting manual entries with phone totals.
