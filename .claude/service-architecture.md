# Service architecture

Read this before changing code. It says which layer owns what, how files are named, and the exact steps for common tasks. Follow the recipes in §8; consistency is what keeps the service easy to change.

Folder-level rules, reasons and gotchas live in `.claude/context/` (see the "Context workflow" in `CLAUDE.md`). This file covers the architecture that applies everywhere.

---

## 1. Stack

- **Language**: Go
- **HTTP**: Gin
- **Store**: MongoDB (Atlas) through the official v2 driver; `STORE=memory` swaps in in-memory repositories for tests and local runs
- **DI**: google/wire, compile-time; regenerate with `make wire`
- **Validation**: ozzo-validation on request DTOs, plus entity allowlists in `pkg/infrastructure/validation/`
- **Logging**: `pkg/logger`, context-first, redacts sensitive keys
- **Testing**: testify + gomock; every interface has a generated mock

---

## 2. Directory layout

```text
cmd/server/main.go               entrypoint: config, DI, router, graceful shutdown

config/                          viper loader + typed *config.Store
assets/                          env JSON configs (dev / stag / prod) and embedded static files

pkg/
├── application/httpserver/      HTTP layer: routing and handlers only, no business logic
│   ├── router.go                engine, middleware chain, version mounts
│   └── v1/public-router/
│       ├── router.go            route registration only
│       ├── bind-helpers.go      binders shared by several handlers
│       └── <action>.go          ONE FILE PER ENDPOINT: list-habits.go, create-habit.go
│
├── domain/
│   ├── dto/request/             request DTOs + Validate(), one file per endpoint
│   ├── dto/response/            response DTOs, one file per endpoint (+ shared objects)
│   ├── bo/                      business objects passed between services
│   ├── helpers/                 pure mappers: DAO/BO → response DTO, no I/O
│   ├── models/dao/              persisted documents (bson tags)
│   └── service/                 IXxxService interfaces + impls; ALL business logic
│
├── infrastructure/
│   ├── mongorepo/               IXxxRepo interfaces + MongoDB impls
│   │   └── memory/              in-memory fakes with identical semantics
│   ├── transport/smtp/          mailers
│   └── validation/              entity field allowlists
│
├── di/
│   ├── container.go             wire provider sets (the only DI file you edit for wiring)
│   ├── repositories.go          picks Mongo or memory repositories
│   ├── wire_gen.go              GENERATED, never edit
│   └── app-logic.go             AppInterface facade used by handlers
│
├── drivers/                     IMongoDB / IPinger
├── middleware/                  request info, recovery, headers, CORS, logging, rate limits, body limit, auth
├── constants/                   limits, TTLs, header and collection names
├── enums/                       one package per enum
├── errors/                      IHttpError + factories, typed repo errors
├── logger/                      redacting logger
└── utils/                       context accessors, dates, cursors, HTTP helpers; scoring/, clock/, ratelimit/

ddl/mongodb/                     documented index set
docs/                            public API contract
```

---

## 3. Layering

```text
Request
  → Middleware  (request info → recovery → secure headers → CORS → request log → IP limit → body limit → auth)
  → Handler     (bind DTO → read ctx values → validate → call service → write JSON)
  → Service     (business rules and orchestration; works in BOs and DAOs)
  → Repository  (MongoDB or memory; returns DAOs)
```

**Hard rules:**
- Handlers never call repositories. Always through a service.
- Services never import `gin` or touch `*gin.Context`. They take `context.Context`.
- Repositories hold no business logic and return only DAOs + `error`.
- Business logic lives only in `pkg/domain/service/`.
- `helpers/` are pure functions: no I/O, no logging, no repository access.
- Lower layers never import higher layers (`mongorepo` must not import `service`).

---

## 4. Model types

| Type | Package | Crosses | Tags |
|---|---|---|---|
| **DTO** | `domain/dto/request`, `domain/dto/response` | client ↔ handler | JSON + ozzo-validation |
| **BO** | `domain/bo` | handler/middleware → service, service ↔ service | none |
| **DAO** | `domain/models/dao` | repository ↔ database | bson |

Flow: handler binds a request **DTO** → service loads **DAOs** from repositories → builds **BOs** when calling another service → `helpers` map DAOs/BOs to the response **DTO**.

Rules:
- When service A calls service B with computed values, B's method takes a BO, never A's request DTO.
- Bundle many parameters into one BO instead of long parameter lists.
- BOs carry no JSON, bson or validation tags.
- `bo.Actor` (user + membership, resolved from the database by the auth middleware) is the one BO handlers pass on, read with `utils.GetActor(ctx)`.

---

## 4.1 File naming

All Go file names are kebab-case. One concern per file.

| Layer | Convention | Example |
|---|---|---|
| Handlers | one file per endpoint, named after the action | `create-habit.go`, `list-chat-sessions.go` |
| Route registry | `router.go`: registration only | `v1/public-router/router.go` |
| Services | `<domain>-service.go`: interface + impl + constructor | `habit-service.go` |
| Repositories | `<domain>-repo.go`: interface + impl + constructor | `user-repo.go` |
| BOs | `<domain>-bo.go` | `actor-bo.go` |
| Helpers | `<domain>-helper.go` | `chat-helper.go` |
| DAOs | `<domain>.go` under `models/dao/` | `dao/user.go` |
| DTOs | one file per endpoint under `dto/request/` and `dto/response/` | `request/signup-request.go` |
| Shared DTO objects | own file when used by several endpoints | `response/user.go`, `request/entity-payload.go` |
| Enums | one lowercase package per enum | `enums/periodflow/` |
| Generated | `wire_gen.go`, `mock_gen.go`: never hand-edit | |

A response-less endpoint (204) has no response file.

---

## 5. Dependency injection

| File | Role | Edit? |
|---|---|---|
| `pkg/di/container.go` | provider sets + `InitializeApp` / `InitializeTestApp` (`//go:build wireinject`) | yes |
| `pkg/di/repositories.go` | `Repositories` bundle; Mongo or memory by `STORE` | yes, for new repositories |
| `pkg/di/wire_gen.go` | generated graph | never, run `make wire` |
| `pkg/di/app-logic.go` | `AppInterface` + `appLogic` struct with accessors | yes, to expose a service |

**DI workflow:**

```text
1. Write the constructor: func NewXxx(log, cfg?, deps...) IXxx
2. Add it to the matching wire.NewSet in pkg/di/container.go
3. make wire
4. If handlers need it: add GetXxx() to AppInterface, a field to appLogic, and the accessor in app-logic.go
```

`appLogic` is built with `wire.Struct(new(appLogic), "*")`, so a new field is filled automatically.

Handlers reach services through `di.AppLogic.GetXxxService()`.

---

## 6. Patterns per layer

### 6.1 Handler

```go
// HandlePatchHabit is PATCH /v1/habits/:id (optimistic version required).
func HandlePatchHabit(c *gin.Context) {
	ctx := c.Request.Context()
	req, ok := bindPatch(c, c.Param("id"))        // 1. bind (BindStrictJSON) + 2. validate
	if !ok {
		return
	}
	resp, herr := di.AppLogic.GetHabitService(). // 3. delegate, actor from ctx
		Patch(ctx, utils.GetActor(ctx), req)
	if herr != nil {
		utils.RespondError(c, herr)              // 4. error envelope (+ Retry-After)
		return
	}
	c.JSON(http.StatusOK, resp)                  // 5. respond
}
```

- JSON bodies are always bound with `utils.BindStrictJSON` (unknown fields and trailing data are rejected).
- Errors are always written with `utils.RespondError`.
- Register the route in `v1/public-router/router.go` in the right access group (public, authed, owner).

### 6.2 Service

```go
type IHabitService interface {
	List(ctx context.Context, actor *bo.Actor, req *request.ListHabitsReq) (*response.ItemsResponse, apperrors.IHttpError)
}

func NewHabitService(log logger.ILogger, entities mongorepo.IEntityRepo, sync ISyncService) IHabitService {
	return &habitService{...}
}
```

- Constructor takes logger, config, repositories and other services; returns the interface.
- Methods return `(result, apperrors.IHttpError)`. The service decides the HTTP status through the factories in `pkg/errors`.
- Infrastructure failures go through `internalErr(...)`: logged, returned as a generic 500, error text never leaked.
- Owner-only writes are checked here too (`requireOwner`), not just in middleware.

### 6.3 Repository

```go
// pkg/infrastructure/mongorepo/user-repo.go
type IUserRepo interface {
	ByID(ctx context.Context, id string) (*dao.User, error)
	Save(ctx context.Context, u *dao.User) error
}
```

- One interface per aggregate, in the same file as its MongoDB implementation.
- Constructors take `drivers.IMongoDB` + logger.
- Missing records return `apperrors.ErrRepoNotFound`; version or uniqueness clashes return `apperrors.ErrRepoConflict`. Services translate them to `IHttpError`.
- Every repository has a matching fake in `memory/` with the same semantics; tests and `STORE=memory` depend on it.
- New indexes go in `EnsureIndexes` and in `ddl/mongodb/indexes.js`.

### 6.4 Errors

```go
return nil, apperrors.NotFoundError("habit not found") // service
utils.RespondError(c, herr)                          // handler
```

Never `panic` for control flow; never return a raw `error` from a service to a handler. Error codes and the envelope are fixed by `docs/api-contract.md`.

### 6.5 Logging

```go
s.log.Error(ctx, "chat session create failed", map[string]any{"err": err.Error(), "userId": actor.User.ID, "sessionId": id})
```

`ctx` is always first. Include `userId` and entity ids. Never log tokens, OTPs, wellness values, period or gender data, chat text or entity `Data` (see `_shared/privacy-invariants.md`).

### 6.6 Config

- Typed fields in `config/`, loaded from `assets/<env>-config.json` by `ACTIVE_ENV`.
- New setting: add the field and its value in all three env files.
- Secrets come only from environment variables, never from the JSON files.
- Business code reads config from the injected `*config.Store`, never from the environment.

### 6.7 Request context

Middleware puts request id, device id, client type, app version, timezone and the resolved actor into `context.Context`. Read them with `utils.GetFromContext` / `utils.GetActor`. Never re-parse headers in handlers or services.

---

## 7. Testing

- Service unit tests use gomock repositories (`createXxxService(ctrl, setup)` helpers) and testify.
- HTTP tests in `pkg/application/httpserver/` build the real graph with `di.InitializeTestApp` over memory repositories, a fake clock and a capturing mailer.
- `contract_test.go` pins the public contract; it must pass unchanged unless the contract changes.
- Table-driven tests, `_test.go` next to the file under test.
- Regenerate mocks after any interface change: `make mock-service`, `make mock-mongorepo`, `make mock-drivers`, `make mock-smtp` (or `make mock-all`). New interfaces must be added to `SERVICE_IFACES` / `MONGOREPO_IFACES` in the `Makefile`.

---

## 8. Recipes

### 8.1 Add an endpoint

1. `pkg/domain/dto/request/<action>.go`: request DTO + `Validate()`.
2. `pkg/domain/dto/response/<action>.go`: response DTO (skip for 204).
3. `pkg/domain/service/<domain>-service.go`: add the method to the interface and the impl.
4. `pkg/domain/helpers/<domain>-helper.go`: mapper to the response DTO, if any.
5. `make mock-service`.
6. `pkg/application/httpserver/v1/public-router/<action>.go`: `HandleXxx` following §6.1.
7. Register it in `router.go`.
8. Tests: service with mocked repositories; HTTP test if routing or access changes.
9. Update `docs/api-contract.md` and the route table in `.claude/context/.../public-router/context.md`.

### 8.2 Add a service

1. `pkg/domain/service/<domain>-service.go`: interface, struct, constructor.
2. Add the constructor to `services` in `pkg/di/container.go`.
3. Add the accessor in `pkg/di/app-logic.go`.
4. `make wire`; add the interface to `SERVICE_IFACES`; `make mock-service`.
5. Add it to the service table in `.claude/context/pkg/domain/service/context.md`.

### 8.3 Add a repository

1. `pkg/domain/models/dao/<domain>.go`: the document.
2. `pkg/infrastructure/mongorepo/<domain>-repo.go`: interface + MongoDB impl; indexes in `EnsureIndexes` and `ddl/mongodb/indexes.js`.
3. `pkg/infrastructure/mongorepo/memory/<domain>.go`: the fake.
4. Add a field to `Repositories` and fill it in both builders in `pkg/di/repositories.go`; add the name to `repositoryFields` in `container.go`.
5. `make wire`; add the interface to `MONGOREPO_IFACES`; `make mock-mongorepo`.
6. If the data is per user, make account erase (`IAccountRepo`) remove it.

### 8.4 Add a config setting

1. Typed field in `config/`.
2. Value in every `assets/*-config.json` (secrets: environment only).
3. Read it through the injected `*config.Store`.

---

## 9. Definition of done

- [ ] `make wire` after any constructor or `container.go` change; `wire_gen.go` committed.
- [ ] Mocks regenerated after any interface change.
- [ ] `make test` passes (`go vet` + `go test -race`).
- [ ] Logic is in services; handlers and repositories stayed thin; helpers are pure.
- [ ] Services return `IHttpError`; handlers use `utils.RespondError`.
- [ ] Logs carry `ctx`, `userId` and entity ids and nothing private.
- [ ] Config values added to all env files.
- [ ] Public contract unchanged, or `docs/api-contract.md` and the app updated with it.
- [ ] `.claude/context/` updated for new or changed rules.
- [ ] No dead code, commented-out code or comments that only restate the name.
