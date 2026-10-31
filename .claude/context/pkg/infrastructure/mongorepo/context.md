# pkg/infrastructure/mongorepo/

## Purpose
Repository interfaces and MongoDB implementations, with in-memory fakes in `memory/` and `mock_gen.go`.

## Rules
- One interface per aggregate in `interfaces.go`.
- Constructor shape: `NewXxxRepo(db drivers.IMongoDB, log)`.
- Return `apperrors.ErrRepoNotFound` / `ErrRepoConflict`.
- Mirror every behavior change in `memory/`, which backs the integration tests.

## Related
`_shared/testing-limits.md`, `ddl/mongodb/context.md`
