# CLAUDE.md — bubu-bloom-service

Go REST + offline-sync API for the Bubu Bloom wellness tracker (two invite-only users: one **owner** who writes, one **viewer** who reads). Public contract: `docs/api-contract.md` (paths `/v1/...`, error body `{"error":{code,message,details}}`) — it must not change without updating the contract and the app. Module path: `github.com/bubu-bloom/service`. Store: MongoDB (Atlas); `STORE=memory` for tests/dev.

The layering follows the mashvara-service architecture (handlers → interface-based services → repository interfaces, Wire DI, `IHttpError`, header middleware, viper env config, Makefile mock/wire targets). Code is original; no proprietary modules are used.

## Commands
```
make build            # bin/http-server + bin/seed
make run              # go run ./cmd  (ACTIVE_ENV=DEV, see .env.example)
make seed             # idempotent provisioning (STORE=mongo)
make wire             # regenerate pkg/di/wire_gen.go from pkg/di/container.go
make mock-service     # pkg/domain/service/mock_gen.go
make mock-mongorepo   # pkg/infrastructure/mongorepo/mock_gen.go
make mock-drivers     # pkg/drivers/mock_gen.go   (also: make mock-smtp, make mock-all)
make test             # go vet ./... && go test -race -count=1 ./...
make test-coverage    # coverage.out + total
```
Regenerate wire/mocks and commit `wire_gen.go` / `mock_gen.go` whenever a constructor signature or an interface changes. Tools install themselves (`make install-tools`: mockgen v1.6.0, wire).

Local run without MongoDB:
`ACTIVE_ENV=DEV STORE=memory ACCESS_TOKEN_SECRET=$(openssl rand -base64 48) OWNER_EMAIL=anu@example.com VIEWER_EMAIL=aman@example.com go run ./cmd` — the OTP is printed in the log (console mailer).

## Layout
```
cmd/http-server.go            entrypoint: wire graph, gin server, graceful shutdown, dev auto-seed
cmd/seed/main.go              idempotent provisioning of owner, viewer, space, 8 default habits, settings
config/                       viper loader (config.Store); assets/{dev,stag,prod}-config.json embedded via assets/embed.go
pkg/application/httpserver/   router.go (middleware chain, 404/405) + v1/public-router/*.go (handlers)
pkg/di/                       container.go (wire sets, build tag wireinject), wire_gen.go (generated),
                              app-logic.go (AppInterface + global AppLogic), repositories.go (mongo|memory selection)
pkg/domain/service/           I*Service interfaces + implementations (+ mock_gen.go, *_test.go with gomock)
pkg/domain/serviceBos/        Actor, ApplyOp, ApplyResult, Person, ProvisionOptions
pkg/domain/models/dao/        persisted documents (bson-tagged)
pkg/domain/dto/{request,response}/   DTOs; every request DTO has Validate() (ozzo-validation)
pkg/infrastructure/mongorepo/ repository interfaces + MongoDB impls; memory/ = in-memory fakes; mock_gen.go
pkg/infrastructure/transport/smtp/   IMailer: smtp, console (dev), capture (tests)
pkg/infrastructure/validation/ strict allowlist entity validation (habit, dailyLog, workout, settings)
pkg/middleware/               generic-request-info, recovery, secure headers, cors, request logger, body limit, rate limit, auth/role
pkg/errors/                   IHttpError + constructors; typed repo errors (ErrRepoNotFound, ErrRepoConflict)
pkg/enums/<name>/             one package per enum (userrole, entitytype, trackingtype, logstatus, workouttype, theme, ...)
pkg/constants/                limits, TTLs, header names, collection names
pkg/drivers/                  IMongoDB / IPinger wrapper (+ mock_gen.go)
pkg/logger/                   ILogger (ctx-first, map fields) over slog with redaction of token/otp/secret-like keys
pkg/utils/                    context helpers, dates, cursor, HTTP helpers; scoring/ (streak rules, fixtures in testdata/scoring.json); clock/; ratelimit/
ddl/mongodb/indexes.js        documented index set (EnsureIndexes creates the same at startup)
```

## Conventions
- **Handler** (`pkg/application/httpserver/v1/public-router`): `ctx := c.Request.Context()`; bind with `utils.BindStrictJSON` (rejects unknown fields) → fill DTO → `req.Validate()` → `di.AppLogic.GetXxxService().Method(ctx, actor, &req)` → `c.JSON(status, resp)` or `utils.RespondError(c, err)`. Actor comes from `utils.GetActor(ctx)`. No logic in handlers.
- **Service**: interface `IXxxService` + `NewXxxService(logger, cfg?, repos…) IXxxService`; methods return `(result, apperrors.IHttpError)`. Infrastructure failures are logged with `internalErr(...)` and surface as a generic 500 (never leak err text). Business outcomes of writes (`conflict`, `rejected`) are `ApplyResult` values, mapped to HTTP by `resultToEntity`.
- **Single write path**: all entity writes (REST and `/sync/push`) go through `ISyncService.Apply`: owner check → operationId idempotency (`IOpRepo`) → strict validation (`validation.Merge`) → compare-and-swap on `version` → LWW rule for daily logs. Habit/Workout/DailyLog/Settings services are thin wrappers (`entity_writer.go`).
- **Repository**: interface per aggregate in `mongorepo/interfaces.go`; constructor `NewXxxRepo(db drivers.IMongoDB, log)`; return `apperrors.ErrRepoNotFound` / `ErrRepoConflict`. Every behavior change must be mirrored in `mongorepo/memory`, which backs the integration tests.
- **DI**: add the constructor to `services` in `pkg/di/container.go`, add the field + accessor in `app-logic.go`, run `make wire`.
- **Errors**: only `pkg/errors` constructors; codes are fixed by the contract (`validation_failed` 400, `unauthorized`/`token_expired` 401, `forbidden` 403, `not_found` 404, `conflict` 409 with `details.server`, `rate_limited` 429 + `Retry-After`, `internal` 500).
- **Tests**: testify; services use `createXxxService(ctrl, setup)` helpers over gomock repos; HTTP behavior is covered by `pkg/application/httpserver/*_test.go` (real gin router + real services + memory repos). `api_test.go` is the original regression suite and its assertions must not be edited to make a change pass.

## Security invariants (do not weaken)
- No signup: accounts exist only via provisioning; `otp/request` answers 202 for any valid email and never reveals membership.
- OTP: 6 digits, HMAC-hashed at rest, 10 min TTL, 5 attempts, resend cooldown, per-email limits; refresh tokens hashed, rotated, family revoked on reuse; access tokens HS256 15 min.
- Role and space are resolved from the database on every request (`IAuthService.Authenticate`); never from the token or the client. Client-supplied `role`, `userId`, `sharedSpaceId`, `createdBy`, `updatedBy` are rejected (unknown field → 400).
- Viewer is read-only: `RequireOwner` middleware **and** service-level `requireOwner` guards.
- Strict JSON (`DisallowUnknownFields`, no trailing data), 1 MiB body limit, secure response headers, rate limits (per IP, per IP on `/auth`, per email).
- Logs never contain tokens, OTPs or wellness values (logger redacts sensitive keys; request log records route patterns only).
- Secrets (`MONGODB_URI`, `ACCESS_TOKEN_SECRET`, `SMTP_*`) come only from the environment. Never commit real values; `assets/*-config.json` hold non-secret settings only. PROD refuses `STORE=memory` and the console mailer.

## Testing limits
MongoDB repositories (`pkg/infrastructure/mongorepo/*.go`) have no automated tests here (no mongod/Docker in CI). Exercise them against a local `mongod` or Atlas before relying on them; the memory fakes mirror their semantics but do not prove driver behavior (BSON mapping, unique indexes, `$inc` counters).
