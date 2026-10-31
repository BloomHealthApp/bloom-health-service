# External services and dependencies

The service makes no outbound HTTP calls. Its only runtime external services are MongoDB and SMTP.

## Runtime services
| Service | Used for | Code | Configuration |
|---|---|---|---|
| MongoDB (Atlas) | All persistent data | `pkg/drivers/mongodb.go`, `pkg/infrastructure/mongorepo/` | `STORE`, `MONGODB_URI` (secret), `MONGODB_DB` |
| SMTP server | OTP emails | `pkg/infrastructure/transport/smtp/` | `MAILER`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER` and `SMTP_PASS` (secret), `MAIL_FROM` |

### MongoDB
- **Writes:** no multi-document transactions. Writes are single-document compare-and-swap on `version`, so any Atlas tier works.
- **Startup:** the service connects, pings and runs `EnsureIndexes`. It needs `createIndex` rights, or see `ddl/mongodb/indexes.js`.
- **Access:** use a dedicated `readWrite` user on this database only, and restrict Atlas Network Access to the host's egress IPs.
- **Automatic expiry (TTL indexes):** `appliedOperations` after 60 days, plus `otpChallenges` and `refreshSessions` at `expiresAt`.
- **Collections:** `users`, `sharedSpaces`, `memberships`, `habits`, `dailyHabitLogs`, `workoutSessions`, `settings`, `periods`, `appliedOperations`, `otpChallenges`, `refreshSessions`, `counters`.
- **Names:** the database (`bubu_bloom`) and JWT issuer (`bubu-bloom`) keep an older project name. Renaming them isn't cosmetic: it would point the service at a different database or invalidate access tokens.
- **Testing:** see `_shared/testing-limits.md`.

### SMTP
- `MAILER=smtp` is required in PROD, and `MAILER=console` prints codes to the log for local use.
- No provider is fixed in the code. A provider API would be a new `smtp.IMailer` implementation.
- `prod-config.json` has a `no-reply@localhost` sender, so set `MAIL_FROM` in production.
- `net/smtp` is used, and it doesn't take the request's context, so there is no per-send timeout from the caller.

## Clients
| Client | Notes |
|---|---|
| `bloom-health-app` (Expo) | Reaches the API at `EXPO_PUBLIC_API_BASE_URL`. Never talks to MongoDB. Shares `docs/api-contract.md`, and keeps a copy of `testdata/scoring.json` at `src/lib/__fixtures__/scoring.json`. |

The app's fallback URL is `http://localhost:8080`, but this service defaults to port 3000, so set the variable for local runs.

## Build and delivery
| Tool | Notes |
|---|---|
| GitHub Actions (`.github/workflows/ci.yml`) | vet, race tests and build, on push to `main` and on pull requests |
| Docker (`Dockerfile`) | Distroless image, `ACTIVE_ENV=PROD`, exposes 3000 |

## Main Go libraries
| Library | Used for |
|---|---|
| `gin-gonic/gin` | HTTP router |
| `go.mongodb.org/mongo-driver/v2` | MongoDB client |
| `google/wire` | Compile-time dependency injection |
| `spf13/viper` | Configuration |
| `go-ozzo/ozzo-validation/v4` | Request DTO validation |
| `golang-jwt/jwt/v5` | Access tokens |
| `golang.org/x/time` | Token-bucket rate limits |
| `golang/mock`, `stretchr/testify` | Tests |

## Known issues
- Rate limiters are in-process (`pkg/utils/ratelimit`). Each running instance has its own counts, and a restart clears them.
