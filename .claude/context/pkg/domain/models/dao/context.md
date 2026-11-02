# pkg/domain/models/dao/

## Purpose
Persisted documents (bson-tagged).

## Gotchas
Legacy users have no phone field, so `User.Phone` can be nil.
`ChatSession` and `ChatMessage` are shared with bloom-health-task-worker (`docs/chat-contract.md`); renaming a bson field breaks the worker.
