# config/

## Purpose
Viper loader producing `config.Store`: `assets/{dev,stag,prod}-config.json` (embedded through `assets/embed.go`, selected by `ACTIVE_ENV`) plus environment overrides.

## Rules
- The JSON files hold non-secret settings only. Secrets come only from the environment.
- `Store.Validate` holds the production safety rules: PROD refuses `STORE=memory` and any mailer except `smtp`, because the console mailer prints OTP codes.

## Gotchas
An environment override only works if its variable is listed in `envBindings`. `CORS_ORIGINS` (comma-separated browser origins) is empty by default, which turns CORS headers off.

## Related
`_shared/security-invariants.md`
