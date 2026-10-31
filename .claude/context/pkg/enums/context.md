# pkg/enums/

## Purpose
One package per enum, each exposing its allowed values. This file covers all of them.

## Rules
Follow the existing shape: constants, `Values()` and `Valid(s)`.

## Gotchas
`avatar` differs: its allowlist is `bubu-01` to `bubu-15`, exposed through `All()` and `IsValid(s)`.
