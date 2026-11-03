# ddl/mongodb/

## Purpose
`indexes.js` documents the index set and can provision a database manually.

## Rules
Keep it in step with `EnsureIndexes` in `pkg/infrastructure/mongorepo/common.go`, which creates the same indexes in the background after startup.
The chat indexes include the worker's claim index (`status, runAt, createdAt`); keep it even though this service never queries by status.
