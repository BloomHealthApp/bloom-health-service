# CLAUDE.md — bloom-health-service

Go REST + offline-sync API for the Bloom Health wellness tracker (passwordless: sign up with name/phone/email, sign in with phone or email, always via an emailed code; each new user gets a private space as **owner**; **viewer** access exists in the data model but has no invite endpoint yet). Public contract: `docs/api-contract.md` (paths `/v1/...`, error body `{"error":{code,message,details}}`) — it must not change without updating the contract and the app. Module path: `github.com/BloomHealthApp/bloom-health-service`. Store: MongoDB (Atlas); `STORE=memory` for tests/dev.

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

## Context workflow (every task)
Project context lives in `.claude/context/`, mirroring the service's folders. Not in code comments.

1. Before changing code in a folder, read the matching `.claude/context/<path>/context.md`, and its nearest parent if there is none. Also read the note for the file you're changing, if there is one: `public-router/<api>.md` for an API, `service/<name>_service.md` for a service.
2. After the change, in the same task, update the context file for any new or changed rule, reason, gotcha or architecture decision. Create the file if the folder now has real context. Delete statements that are no longer true.
3. New, moved or deleted service folders: make the same change in `.claude/context/`. New or changed routes and services: update their note and the index table in that folder's `context.md`. New external services or changes to how one is used: update `_shared/external-services.md`.
4. Do not add comments to code to hold context, history or task notes.
   Allowed in code: godoc that says more than the name; a short why-comment for non-obvious code; a brief warning where a security or privacy rule could be broken; build directives.
5. Skip the update when nothing new was learned (e.g. a typo fix). Say in the final message which context files changed, or that none needed to.

## Where things are
Paths with context have a matching file at `.claude/context/<path>/context.md`. `.claude/context/_shared/testing-limits.md` covers what the tests can't prove, and `_shared/external-services.md` covers MongoDB, SMTP, clients and tooling. Every route is indexed in `.claude/context/pkg/application/httpserver/v1/public-router/context.md`, and every service in `.claude/context/pkg/domain/service/context.md`.
```
cmd/server/                    entrypoint
config/  assets/               configuration
ddl/mongodb/                   documented index set
pkg/application/httpserver/    router, HTTP tests; v1/public-router/ = handlers
pkg/di/                        Wire graph and AppLogic
pkg/domain/service/            services
pkg/domain/serviceBos/         business objects
pkg/domain/dto/                request and response DTOs
pkg/domain/models/dao/         persisted documents
pkg/enums/                     one package per enum
pkg/errors/                    IHttpError and typed repo errors
pkg/infrastructure/mongorepo/  repositories; memory/ = in-memory fakes
pkg/infrastructure/transport/smtp/   mailers
pkg/infrastructure/validation/ entity allowlists
pkg/logger/                    redacting logger
pkg/middleware/                middleware chain
pkg/constants/                 limits, TTLs, header names, collection names
pkg/drivers/                   IMongoDB / IPinger wrapper (+ mock_gen.go)
pkg/utils/                     context helpers, dates, cursor, HTTP helpers; scoring/, clock/, ratelimit/
```

## Invariants (always apply)
@.claude/context/_shared/security-invariants.md
@.claude/context/_shared/privacy-invariants.md
