# Testing limits

MongoDB repositories (`pkg/infrastructure/mongorepo/*.go`, including the `periods` collection and its indexes) have no automated tests here (no mongod/Docker in CI). Exercise them against a local `mongod` or Atlas before relying on them; the memory fakes mirror their semantics but do not prove driver behavior (BSON mapping, unique indexes, `$inc` counters).
