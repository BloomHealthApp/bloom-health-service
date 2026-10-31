# CLAUDE.md — bloom-health-service

Go REST + offline-sync API for the Bloom Health wellness tracker (passwordless: sign up with name/phone/email, sign in with phone or email, always via an emailed code; each new user gets a private space as **owner**; **viewer** access exists in the data model but has no invite endpoint yet). Public contract: `docs/api-contract.md` (paths `/v1/...`, error body `{"error":{code,message,details}}`) — it must not change without updating the contract and the app. Module path: `github.com/bubu-bloom/service`. Store: MongoDB (Atlas); `STORE=memory` for tests/dev.

The service uses a layered clean architecture (handlers → interface-based services → repository interfaces, Wire DI, `IHttpError`, header middleware, viper env config, Makefile mock/wire targets).

## Commands
```
make build            # bin/http-server
make run              # go run ./cmd/server  (ACTIVE_ENV=DEV, see .env.example)
make wire             # regenerate pkg/di/wire_gen.go from pkg/di/container.go
make mock-service     # pkg/domain/service/mock_gen.go
make mock-mongorepo   # pkg/infrastructure/mongorepo/mock_gen.go
make mock-drivers     # pkg/drivers/mock_gen.go   (also: make mock-smtp, make mock-all)
make test             # go vet ./... && go test -race -count=1 ./...
make test-coverage    # coverage.out + total
```
Regenerate wire/mocks and commit `wire_gen.go` / `mock_gen.go` whenever a constructor signature or an interface changes. Tools install themselves (`make install-tools`: mockgen v1.6.0, wire).

Local run without MongoDB:
`ACTIVE_ENV=DEV STORE=memory ACCESS_TOKEN_SECRET=$(openssl rand -base64 48) go run ./cmd/server` — the OTP is printed in the log (console mailer).

## Layout
```
cmd/server/main.go             entrypoint: wire graph, gin server, graceful shutdown
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
pkg/infrastructure/validation/ strict allowlist entity validation (habit, dailyLog, workout, settings, period)
pkg/middleware/               generic-request-info, recovery, secure headers, cors, request logger, body limit, rate limit, auth/role
pkg/errors/                   IHttpError + constructors; typed repo errors (ErrRepoNotFound, ErrRepoConflict)
pkg/enums/<name>/             one package per enum (userrole, entitytype, trackingtype, logstatus, workouttype, theme, avatar (bubu-01..bubu-09 allowlist), gender, periodflow, periodmood, periodsymptom, ...)
pkg/constants/                limits, TTLs, header names, collection names
pkg/drivers/                  IMongoDB / IPinger wrapper (+ mock_gen.go)
pkg/logger/                   ILogger (ctx-first, map fields) over slog with redaction of token/otp/secret-like keys
pkg/utils/                    context helpers, dates, cursor, HTTP helpers; scoring/ (streak rules, pure: Settings.NeutralDates behave like rest days; fixtures in testdata/scoring.json); clock/; ratelimit/
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
- Auth is OTP-only (no passwords). `signin/request` answers 202 `{status, challengeId}` for every well-formed identifier (email or phone) and never reveals whether an account exists: unknown identifiers get a random decoy `challengeId` and no mail; the code is always mailed to the email on the account. `signup/request` returns `409 already_registered` for a taken email/phone (accepted trade-off, rate limited). The account, private space, default habits and settings are created only after a sign-up challenge is verified; the pending name/phone live in the challenge, never in `users`.
- Phone: normalized to E.164 by `utils.NormalizePhone` (default country code from `DEFAULT_COUNTRY_CODE`, `+91`), unique via a partial unique index on `users.phone` (legacy users have no phone field); unique-index races map to `ErrRepoConflict` -> 409 `already_registered`. Phones are unverified lookup keys.
- OTP: 6 digits, HMAC-hashed at rest (bound to the challengeId), keyed by `challengeId` (single use, deleted on success), 10 min TTL, 5 attempts, resend cooldown per account (`IOTPRepo.LatestByEmail`), per-identifier and per-challenge limits; refresh tokens hashed, rotated, family revoked on reuse; access tokens HS256 15 min.
- Role and space are resolved from the database on every request (`IAuthService.Authenticate`); never from the token or the client. Client-supplied `role`, `userId`, `sharedSpaceId`, `createdBy`, `updatedBy` are rejected (unknown field → 400).
- Viewer is read-only: `RequireOwner` middleware **and** service-level `requireOwner` guards.
- Strict JSON (`DisallowUnknownFields`, no trailing data), 1 MiB body limit, secure response headers, rate limits (per IP, per IP on `/auth`, per normalized email/phone, per challenge on verify).
- Logs never contain tokens, OTPs or wellness values (logger redacts sensitive keys; request log records route patterns only).
- Secrets (`MONGODB_URI`, `ACCESS_TOKEN_SECRET`, `SMTP_*`) come only from the environment. Never commit real values; `assets/*-config.json` hold non-secret settings only. PROD refuses `STORE=memory` and the console mailer.

## Period tracker and neutral days (privacy invariants, do not weaken)
- Period entries (entityType `period`, collection `periods`, REST `/periods*`) are **owner-only health data**: viewers get 403 on `/periods*`, `ISyncService.Pull` filters `period` rows out for non-owners (`changes.periods` is `[]`, cursor still advances), viewer pushes are rejected, and no other endpoint returns period rows. Do not add a code path that reads `entitytype.Period` for a viewer; stats only expose the generic `restDay` flag.
- The tracker is on when `User.IsPeriodTrackerEnabled()`: the explicit `periodTracker` setting if set, else `gender == female`. When off, `/periods*` and period ops (`ISyncService.Apply`) are refused (403 / rejected `period_tracker_disabled`) and period days stop being neutral; data is kept.
- Cross-record rules (no overlap, one ongoing) live in `syncService.checkPeriodRules`, because they need sibling rows; field rules are in `validation.validatePeriod`.
- Streak neutrality is computed in `statsService`: `RestDays = settings.restDays + {0 when sundayRest true/absent}`, `NeutralDates` = owner's non-deleted periods (ongoing = through today) only if the owner's tracker is enabled. The scoring library stays pure.
- Logs must never contain period/gender data (logger redacts `period`, `gender`, `symptom`, `mood`, `flow`, `startdate`, `enddate` keys). Never log entity `Data`.

## Testing limits
MongoDB repositories (`pkg/infrastructure/mongorepo/*.go`, including the new `periods` collection and its indexes) have no automated tests here (no mongod/Docker in CI). Exercise them against a local `mongod` or Atlas before relying on them; the memory fakes mirror their semantics but do not prove driver behavior (BSON mapping, unique indexes, `$inc` counters).
