# pkg/di/

## Purpose
- `container.go`: wire sets (build tag `wireinject`).
- `wire_gen.go`: generated.
- `app-logic.go`: `AppInterface` and the global `AppLogic`.
- `repositories.go`: mongo or memory selection.

## Rules
To add a service: add the constructor to `services` in `container.go`, add the field and accessor in `app-logic.go`, then run `make wire` and commit `wire_gen.go`.
