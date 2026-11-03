# pkg/domain/helpers/

## Purpose
Pure mappers from DAOs and BOs to response DTOs: chat sessions and messages, entity lists, sync results, the `user` object.

## Rules
- No I/O, no logging, no repository or config access. Anything that needs a lookup stays in the service, which passes the result in (`buildUserResponse` finds the owner, then calls `UserResponse`).
- No business rules or HTTP status decisions (`resultToEntity` stays in `service/entity-writer.go`).
- Table-driven test next to each file.
