# pkg/domain/dto/

## Purpose
`request/` and `response/` DTOs. JSON field names follow `docs/api-contract.md`.

## Rules
- Every request DTO has `Validate()` built on ozzo-validation.
- One file per endpoint, named like its handler (`request/signup-request.go`, `response/list-chat-sessions.go`).
- Types used by several endpoints have their own file: `request/entity-payload.go` (habit, workout and period create/patch), `request/date-range.go`, `request/refresh-token.go`, `request/optional.go`, `request/identity.go` (email and phone helpers); `response/user.go`, `response/auth.go`, `response/challenge.go`, `response/status.go`, `response/items.go`, `response/chat-session.go`, `response/chat-message.go`.
- 204 endpoints have no response file.
