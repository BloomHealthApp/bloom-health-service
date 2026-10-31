# Service context

Context for working on this service. It lives here so the Go code stays readable. The rules for reading and updating it are in the "Context workflow" section of `CLAUDE.md`.

- Paths mirror the service: `pkg/middleware/` is described by `.claude/context/pkg/middleware/context.md`.
- Only folders with real context get a file. If a folder has none, read its nearest parent.
- A folder's `context.md` can link to notes for individual files, named after the source file. These cover each API (`public-router/auth.md` for `auth.go`) and each service (`service/auth_service.md`).
- `_shared/` holds rules that span many folders and has no service equivalent, including `external-services.md`.
- Generated files (`wire_gen.go`, `mock_gen.go`) are never documented or edited by hand.

## File template
Use these sections and leave out any that are empty. Record what the code can't show, and don't restate it.

```md
# <service path>/

## Purpose
## Rules
## Why
## Gotchas
## Related
```
