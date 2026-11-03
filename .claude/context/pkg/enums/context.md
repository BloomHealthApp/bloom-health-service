# pkg/enums/

## Purpose
One package per enum, each exposing its allowed values. This file covers all of them.

## Rules
Follow the existing shape: constants and `Values()` (used by the validation allowlists). Add `Valid(s)` only when code needs it; enums only compared against (`opstatus`, `chatrole`, `chatstatus`) are constants alone.

## Gotchas
`avatar` differs: its allowlist is `bubu-01` to `bubu-15`, exposed through `All()` and `IsValid(s)`.
`chatstatus.Wire` maps the worker-only `running` to `pending` for clients.
