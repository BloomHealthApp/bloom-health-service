# pkg/application/httpserver/

## Purpose
`router.go` builds the gin engine: middleware chain and 404/405 handling. `*_test.go` covers HTTP behavior with the real gin router, real services and memory repos.

## Rules
`api_test.go` is the original regression suite. Its assertions must not be edited to make a change pass.

## Why
`api_test.go` holds the integration tests from the original chi implementation, run unchanged through gin to prove the refactor kept contract parity.

## Related
`pkg/middleware/context.md`, `docs/api-contract.md`
