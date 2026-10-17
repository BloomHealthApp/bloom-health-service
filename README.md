# Bubu Bloom Service

Go (chi) REST + sync API for the Bubu Bloom wellness tracker. MongoDB Atlas is the cloud source of truth; the mobile app (`../bubu-bloom-app`) never talks to MongoDB directly. Contract: [`../docs/api-contract.md`](../docs/api-contract.md).

> Spec deviation: the original spec suggested Node/Express; this service is Go by explicit decision.

## Layout
```
cmd/server        HTTP server        cmd/seed   idempotent invite-only provisioning
internal/config   env config         internal/store    Store interface: memory (tests/dev) + MongoDB
internal/auth     OTP + JWT + rotating refresh tokens
internal/entities strict allowlist validation (blocks mass-assignment)
internal/service  apply/sync/pull/stats logic    internal/scoring  completion + streak rules
internal/httpapi  router, middleware, handlers + integration tests
testdata/scoring.json  shared scoring fixtures (copied into the app)
```

## Run locally
```
cp .env.example .env   # edit; export the variables (e.g. `set -a; source .env; set +a`)
# Quick start without MongoDB (ephemeral):
STORE=memory ACCESS_TOKEN_SECRET=$(openssl rand -base64 48) OWNER_EMAIL=anu@example.com VIEWER_EMAIL=aman@example.com go run ./cmd/server
# With Atlas: set MONGODB_URI, then
go run ./cmd/seed && go run ./cmd/server
```
With `MAILER=console` the OTP code is printed in the server log (dev only; production refuses to start with it).

Sign in: `POST /v1/auth/otp/request {"email":"anu@example.com"}` → read code from log → `POST /v1/auth/otp/verify`.

## Tests
`make test` (`go vet` + `go test -race`). Covers scoring fixtures, validation, OTP/refresh rotation + reuse detection, viewer read-only, outsider 403, cross-space isolation, mass-assignment, optimistic versions, sync idempotency/conflict/LWW/pull.
The MongoDB store implementation compiles but is **not covered by automated tests** (no MongoDB in CI here); exercise it against Atlas or a local `mongod` before relying on it.

## Atlas / deployment notes
- Create a dedicated DB user with `readWrite` on the `bubu_bloom` database only.
- Restrict Atlas Network Access to your host's egress IPs (avoid 0.0.0.0/0).
- Indexes are created on startup (`EnsureIndexes`): unique memberships, unique `habitId+localDate` logs, TTL on OTPs / refresh sessions / applied operations.
- No multi-document transactions are used; writes rely on single-document compare-and-swap on `version`, so any Atlas tier works.
- Store `MONGODB_URI`, `ACCESS_TOKEN_SECRET`, SMTP creds in the host's secret manager. `.env` is git-ignored.
- `Dockerfile` builds a distroless image. Put it behind TLS and set `TRUST_PROXY=true` only if a trusted proxy sets `X-Forwarded-For`.
- Mailer: `console` and `smtp` implemented; add a provider (e.g. Resend HTTP API) by implementing `mailer.Mailer`.

## Security model
Invite-only (no signup endpoint; only seeded emails get codes, responses never reveal membership), 6-digit OTP hashed with HMAC, 10 min TTL, 5 attempts, resend cooldown, per-IP/per-email rate limits; 15 min HS256 access tokens; rotating refresh tokens (hashed at rest, family revoked on reuse); role and space resolved from the database on every request; strict JSON (unknown fields rejected); logs exclude tokens, OTPs and wellness values.
