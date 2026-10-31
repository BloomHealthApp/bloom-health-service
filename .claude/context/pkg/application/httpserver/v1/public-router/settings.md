# Settings API (`settings.go`)

Service: [settings_service.md](../../../../domain/service/settings_service.md). Contract section: Profile & settings.

| Route | Access | Success |
|---|---|---|
| `GET /settings` | both | 200 `Settings` |
| `PATCH /settings` | owner | 200 `Settings` |

## Gotchas
- The entity id is always `settings`; there is no id in the path.
- A `PATCH` with no `version` isn't a 400 as on other entities. It uses base version 0, which returns 409 when a record exists.
- `GET` returns defaults when no record exists, and adds `sundayRest: true` to legacy records.
