# IDMService (`dm-service.go`)

Text messages between friends ("Messages" in the app) and the Expo push tokens used to announce them, which live on the user document (`users.pushTokens`).

## Rules
- `friend` is the only gate: an accepted connection with a live user, not yourself. Everything else is `errNoChat` (404), so a stranger can't tell a chat apart from an unknown id.
- A thread is one document per pair (`_id` = `dao.PairKey`). `NextSeq` reserves the next message number atomically (`$inc`, upsert), so paging and polling use integers (`before`/`after`) rather than timestamps, which Mongo keeps only to the millisecond.
- The app picks the message id. A resend of the same id by the same sender in the same chat returns the saved message (200, `created=false`); someone else's id is a 409. The id is checked before reserving a seq, so retries don't burn numbers (a lost race still can, which only leaves a gap).
- `SetLast` only moves forward (`last.seq` must be lower), so concurrent sends can't leave an older preview.
- Sending marks the chat read for the sender. `unread` = newest message is from the friend and newer than your `readAt`.
- `Threads` drops chats with anyone who isn't a current friend (an unfriend whose chat deletion failed) or is pending deletion.
- Daily cap: 500 sent per UTC day, counted in the database (`CountSentSince`); the in-memory `LimitMessages` limiter only smooths bursts per instance.

## Push
- `notify` runs inline after the message is saved, with a 3 s timeout: Vercel stops work once the response is sent, so no goroutine. A push failure is logged and never fails the send.
- The body is "<first name> sent you a message". Never add the text: it shows on the lock screen. Data `{type: "dm", from}`; Android channel `messages` (the app creates it).
- Tokens Expo reports as `DeviceNotRegistered` are deleted. A token belongs to one user: saving it pulls it from any other account first, so signing in as someone else on the same phone moves it. `notify` reads them straight from the friend's user document, already loaded by `friend`.

## Related
`pkg/drivers/expo-push.go`, `pkg/infrastructure/mongorepo/direct-message-repo.go`, `user-repo.go` (push tokens), `_shared/privacy-invariants.md`.
