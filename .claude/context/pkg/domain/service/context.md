# pkg/domain/service/

## Purpose
`I*Service` interfaces and implementations, `mock_gen.go`, and `*_test.go` with gomock.

## Rules
- Shape: interface `IXxxService` + `NewXxxService(logger, cfg?, repos…) IXxxService`. Methods return `(result, apperrors.IHttpError)`.
- Infrastructure failures are logged with `internalErr(...)` and surface as a generic 500. Never leak err text.
- Business outcomes of writes (`conflict`, `rejected`) are `ApplyResult` values, mapped to HTTP by `resultToEntity`.
- **Single write path:** all entity writes (REST and `/sync/push`) go through `ISyncService.Apply`: owner check → operationId idempotency (`IOpRepo`) → strict validation (`validation.Merge`) → compare-and-swap on `version` → LWW rule for daily logs.
- Habit/Workout/DailyLog/Settings services are thin wrappers (`entity-writer.go`).
- Tests use testify and `createXxxService(ctrl, setup)` helpers over gomock repos.

## All services
| Service | Note | Depends on |
|---|---|---|
| `IAuthService` | [auth-service.md](auth-service.md) | users, memberships, OTP, sessions, mailer, Provision |
| `IProvisionService` | [provision-service.md](provision-service.md) | users, spaces, memberships, entities |
| `IUserService` | [user-service.md](user-service.md) | users, memberships, accounts (erase) |
| `ISyncService` | [sync-service.md](sync-service.md) | entities, operations |
| `IHabitService` | [habit-service.md](habit-service.md) | entities, Sync |
| `IDailyLogService` | [daily-log-service.md](daily-log-service.md) | entities, Sync |
| `IWorkoutService` | [workout-service.md](workout-service.md) | entities, Sync |
| `IPeriodService` | [period-service.md](period-service.md) | entities, Sync |
| `ISettingsService` | [settings-service.md](settings-service.md) | entities, Sync |
| `IStatsService` | [stats-service.md](stats-service.md) | entities, users, memberships |
| `IHealthService` | [health-service.md](health-service.md) | `drivers.IPinger` |
| `IChatService` | [chat-service.md](chat-service.md) | chat sessions, chat messages, `config.ChatConfig` |

Shared helper: [entity-writer.md](entity-writer.md).

## Related
`_shared/security-invariants.md`, `_shared/privacy-invariants.md`
