# pkg/logger/

## Purpose
`ILogger` (ctx-first, map fields) over slog, with redaction of sensitive keys.

## Rules
Adding a new kind of sensitive field means adding it to `sensitiveParts`.

## Related
`_shared/security-invariants.md`, `_shared/privacy-invariants.md`
