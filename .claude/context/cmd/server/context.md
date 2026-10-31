# cmd/server/

## Purpose
Entrypoint: builds the Wire graph, sets `di.AppLogic`, runs the gin server and shuts down gracefully on SIGINT/SIGTERM.

## Gotchas
`di.AppLogic` must be set before `httpserver.NewRouter()` is called, because the router reads its dependencies from it.
