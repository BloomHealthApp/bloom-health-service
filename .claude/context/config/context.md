# config/

## Purpose
Viper loader producing `config.Store`: `assets/{dev,stag,prod}-config.json` (embedded through `assets/embed.go`, selected by `ACTIVE_ENV`) plus environment overrides.

## Rules
- The JSON files hold non-secret settings only. Secrets come only from the environment.
- `Store.Validate` holds the production safety rules: PROD refuses `STORE=memory` and any mailer except `smtp`, because the console mailer prints OTP codes.

## Gotchas
An environment override only works if its variable is listed in `envBindings`. `CORS_ORIGINS` (comma-separated browser origins) is empty by default, which turns CORS headers off.

- `REVIEW_LOGIN_EMAIL` + `REVIEW_LOGIN_CODE` are read straight from the environment (secrets, not in `envBindings`). `validateReviewLogin` needs both or neither, an email, and a 6-digit code that isn't `123456`, `654321` or one repeated digit. Bad values never stop startup: the review login is turned off, `ReviewLoginProblem` holds the reason, and `cmd/server` logs `review login disabled` with it (never the code). Unlike the required settings, this optional feature must not take the API down (it did once, as a 500 `FUNCTION_INVOCATION_FAILED` on Vercel).

- `INTERNAL_API_SECRET` (env only) authenticates bloom-health-task-worker's `POST /internal/accounts/purge-deleted` (`Authorization: Bearer`, constant-time compare). Empty turns the internal routes off (404). The same value is in the worker's env.

## Related
`_shared/security-invariants.md`

## Chat
The `chat` section (`config/chat.go`) holds the model catalogue (`models[]`) and `dailyMessageCap` (env `CHAT_DAILY_MESSAGE_CAP`, read with `GetInt` because `UnmarshalKey` ignores env bindings). The catalogue must match bloom-health-task-worker's. No model API key belongs in this service.
