# IHealthService (`health_service.go`)

## Purpose
Pings the store through `drivers.IPinger`, and reports status, time and the configured version.

## Rules
The failure detail goes to the log only. Callers get a generic 503.

## Gotchas
With `STORE=memory` the pinger always succeeds, so a passing check there says nothing about MongoDB.
