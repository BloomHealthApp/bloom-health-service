# pkg/logger/

## Purpose
`ILogger` (ctx-first, map fields) over slog, with redaction of sensitive keys.

## Rules
Adding a new kind of sensitive field means adding it to `sensitiveParts`. Chat text keys (`content`, `prompt`, `title`) are redacted, so any field whose name contains them is too.

Every line gets `requestId` from the context and, after auth, `userId` from the actor, so callers only add entity ids. An explicit `userId` field wins over the actor's.

When the context carries a valid span (the `Telemetry` middleware, or a caller's `traceparent` even with export off), lines also get `traceId` and `spanId`, so logs join traces.

## Related
`_shared/security-invariants.md`, `_shared/privacy-invariants.md`
