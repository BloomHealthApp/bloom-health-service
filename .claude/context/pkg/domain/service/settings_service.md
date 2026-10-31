# ISettingsService (`settings_service.go`)

## Purpose
Reads and updates the one settings record per space (id `constants.SettingsID`).

## Gotchas
- `Get` returns unsaved defaults when no record exists, using `validation.DefaultSettings()`.
- `Get` adds `sundayRest: true` to legacy records that don't have the field.
- `Patch` treats a missing version as 0, which conflicts with any existing record.
- `Apply` rejects a delete or any other id for settings (`invalid_settings_operation`).
