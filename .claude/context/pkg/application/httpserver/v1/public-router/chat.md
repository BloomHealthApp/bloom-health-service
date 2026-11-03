# Chat API (`chat.go`)

Service: [chat_service.md](../../../../domain/service/chat_service.md). Contract section: Chat. Queue shapes: `docs/chat-contract.md`.

| Route | Access | Success |
|---|---|---|
| `GET /chat/models` | both | 200 `{items}` from config |
| `POST /chat/sessions` | both | 201 ChatSession |
| `GET /chat/sessions` | both | 200 `{items}`, newest first, at most 100 |
| `GET /chat/sessions/:sid` | both | 200 ChatSession |
| `DELETE /chat/sessions/:sid` | both | 204; erases the messages, clears the title and leaves a tombstone that expires after 7 days |
| `GET /chat/sessions/:sid/messages?before&limit` | both | 200 page, oldest first |
| `POST /chat/sessions/:sid/messages` | both | 202 `{userMessage, assistantMessage}` |

## Rules
- Chat is per user, not per space: viewers may chat, and a session is 404 for everyone but its creator.
- Not under `RequireOwner` and not part of sync.

## Gotchas
- `409` while a reply is pending is the contract's generic `conflict` code with `details.reason = "turn_in_flight"`; no new error code was added.
- The daily cap (`chat.dailyMessageCap`, `CHAT_DAILY_MESSAGE_CAP`) is counted from stored user messages per UTC day, so it holds across instances, unlike the in-process rate limiters.
