# Testing limits

MongoDB repositories (`pkg/infrastructure/mongorepo/*.go`, including the `periods` collection and its indexes) have no automated tests here (no mongod/Docker in CI). Exercise them against a local `mongod` or Atlas before relying on them; the memory fakes mirror their semantics but do not prove driver behavior (BSON mapping, unique indexes, `$inc` counters).

HTTP tests share the production rate limiters, and the per-IP burst is 90 requests. A test that sends many requests, especially one whose count depends on today's date, must call `relaxLimits` or it can fail with 429 on some days only (`TestSundayRestDefault` did on Mondays).

Chat HTTP tests cover the API and enqueueing only. The worker's claim, settle and reaper run in bloom-health-task-worker; `memory.SettleForTest` stands in for it here.
