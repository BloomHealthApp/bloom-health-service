# pkg/domain/service/

## Purpose
`I*Service` interfaces and implementations, `mock_gen.go`, and `*_test.go` with gomock.

## Rules
- Shape: interface `IXxxService` + `NewXxxService(logger, cfg?, repos…) IXxxService`. Methods return `(result, apperrors.IHttpError)`.
- Infrastructure failures are logged with `internalErr(...)` and surface as a generic 500. Never leak err text.
- Business outcomes of writes (`conflict`, `rejected`) are `ApplyResult` values, mapped to HTTP by `resultToEntity`.
- **Single write path:** all entity writes (REST and `/sync/push`) go through `ISyncService.Apply`: owner check → operationId idempotency (`IOpRepo`) → strict validation (`validation.Merge`) → compare-and-swap on `version` → LWW rule for daily logs.
- Habit/Workout/DailyLog/Settings services are thin wrappers (`entity_writer.go`).
- Tests use testify and `createXxxService(ctrl, setup)` helpers over gomock repos.

## All services
| Service | Note | Depends on |
|---|---|---|
| `IAuthService` | [auth_service.md](auth_service.md) | users, memberships, OTP, sessions, mailer, Provision |
| `IProvisionService` | [provision_service.md](provision_service.md) | users, spaces, memberships, entities |
| `IUserService` | [user_service.md](user_service.md) | users, memberships |
| `ISyncService` | [sync_service.md](sync_service.md) | entities, operations |
| `IHabitService` | [habit_service.md](habit_service.md) | entities, Sync |
| `IDailyLogService` | [daily_log_service.md](daily_log_service.md) | entities, Sync |
| `IWorkoutService` | [workout_service.md](workout_service.md) | entities, Sync |
| `IPeriodService` | [period_service.md](period_service.md) | entities, Sync |
| `ISettingsService` | [settings_service.md](settings_service.md) | entities, Sync |
| `IStatsService` | [stats_service.md](stats_service.md) | entities, users, memberships |
| `IHealthService` | [health_service.md](health_service.md) | `drivers.IPinger` |

Shared helper: [entity_writer.md](entity_writer.md).

## Related
`_shared/security-invariants.md`, `_shared/privacy-invariants.md`
