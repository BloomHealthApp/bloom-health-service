# Bubu Bloom Service

Go (gin) REST + sync API for the Bubu Bloom wellness tracker. MongoDB Atlas is the cloud source of truth; the mobile app (`../bubu-bloom-app`) never talks to MongoDB directly. Contract: [`docs/api-contract.md`](docs/api-contract.md) (identical to `../docs/api-contract.md`).

The service follows a clean-architecture layout: **handlers → interface-based services → repository interfaces**, wired with google/wire. See [`CLAUDE.md`](CLAUDE.md) for conventions.

## Layout
```
cmd/server/main.go    HTTP server
config/ + assets/    viper config, per-env JSON (dev|stag|prod), secrets only from env
pkg/application/httpserver   gin router, middleware chain, /v1 handlers
pkg/domain           services (+ gomock mocks), DTOs (ozzo validation), business objects, DAOs
pkg/infrastructure   mongorepo (Mongo repos + memory fakes), smtp mailer, entity validation
pkg/di               wire container and AppLogic accessor
pkg/middleware       request info, recovery, secure headers, CORS, logging, body limit, rate limit, auth
pkg/{errors,enums,constants,drivers,logger,utils}
ddl/mongodb/indexes.js   documented indexes
testdata/scoring.json    shared scoring fixtures (copied into the app)
```

## Run locally
```
cp .env.example .env   # edit; export the variables (e.g. `set -a; source .env; set +a`)
# Quick start without MongoDB (ephemeral):
ACTIVE_ENV=DEV STORE=memory ACCESS_TOKEN_SECRET=$(openssl rand -base64 48) \
  make run
# With Atlas: set STORE=mongo and MONGODB_URI, then
make run
```
With `MAILER=console` (default in DEV) the OTP code is printed in the server log (dev only; PROD refuses to start with it).

Sign up: `POST /v1/auth/signup/request {"name":"Anu","phone":"98765 43210","email":"anu@example.com"}` → `202 {"status":"sent","challengeId":"..."}` → read the code from the log → `POST /v1/auth/otp/verify {"challengeId","code"}` (creates the account) → use `accessToken` as `Authorization: Bearer ...`.
Sign in: `POST /v1/auth/signin/request {"identifier":"anu@example.com"}` (email **or** phone; the code is mailed to the email on the account) → same `otp/verify`. There are no passwords.

## Configuration
`ACTIVE_ENV` (`DEV` default, `STAG`, `PROD`) selects `assets/<env>-config.json` (embedded in the binary). Environment variables override file values (`PORT`, `STORE`, `MONGODB_DB`, `AUTH_ISSUER`, `MAILER`, `SMTP_PORT`, `MAIL_FROM`, `TRUST_PROXY`, `CORS_ORIGINS`, `LOG_LEVEL`, `DEFAULT_TIMEZONE`, `DEFAULT_COUNTRY_CODE`). `DEFAULT_COUNTRY_CODE` (default `+91`) is prefixed to bare 10-digit phone numbers. Secrets are read **only** from the environment: `MONGODB_URI`, `ACCESS_TOKEN_SECRET` (>= 32 chars), `SMTP_HOST`/`SMTP_USER`/`SMTP_PASS`.

## Commands
`make build | run | wire | mock-service | mock-mongorepo | mock-drivers | mock-smtp | mock-all | test | test-coverage`.
`make wire` and the `mock-*` targets regenerate `pkg/di/wire_gen.go` and the `mock_gen.go` files (committed). CI (`.github/workflows/ci.yml`) runs `go vet ./...` and `go test -race ./...`.

## Tests
`make test`. Covers scoring fixtures, entity validation, DTO validation, the original HTTP integration suite run through the gin router with in-memory repos (sign-up/sign-in by challenge, phone uniqueness and normalization, decoy challenges, OTP/refresh rotation + reuse detection, viewer read-only, outsider 403, cross-space isolation, mass-assignment, optimistic versions, sync idempotency/conflict/LWW/pull), error-envelope/rate-limit contract tests, and gomock unit tests for the auth, sync and viewer-guard services.
The MongoDB repositories compile but are **not covered by automated tests** (no MongoDB in CI here); exercise them against Atlas or a local `mongod` before relying on them.

## Atlas / deployment notes
- Create a dedicated DB user with `readWrite` on the `bubu_bloom` database only; restrict Atlas Network Access to your host's egress IPs.
- Indexes are created on startup (`EnsureIndexes`); `ddl/mongodb/indexes.js` documents the same set and can be run with `mongosh` to provision up front.
- No multi-document transactions: writes use single-document compare-and-swap on `version`, so any Atlas tier works.
- Store `MONGODB_URI`, `ACCESS_TOKEN_SECRET`, SMTP credentials in the host's secret manager. `.env` is git-ignored.
- `Dockerfile` builds a distroless image (`ACTIVE_ENV=PROD` by default). Put it behind TLS and set `TRUST_PROXY=true` only if a trusted proxy sets `X-Forwarded-For`.
- Mailer: `console` and `smtp` implemented; add a provider (e.g. an HTTP API) by implementing `smtp.IMailer`.

## Security model
Passwordless sign-up (name, phone, email) and sign-in (phone or email) with an emailed code and an opaque `challengeId`; the account and a private space are created only after the sign-up code is verified; sign-in always answers 202 (unknown identifiers get a random decoy challenge and no mail), while sign-up reports `409 already_registered` for a taken email/phone (rate limited); phone numbers are unverified lookup keys (E.164, unique), the code always goes to the email on file; 6-digit OTP hashed with HMAC, 10 min TTL, 5 attempts, resend cooldown, per-IP/per-email rate limits; 15 min HS256 access tokens; rotating refresh tokens (hashed at rest, family revoked on reuse); role and space resolved from the database on every request; strict JSON (unknown fields rejected, 1 MiB limit); logs exclude tokens, OTPs and wellness values.
