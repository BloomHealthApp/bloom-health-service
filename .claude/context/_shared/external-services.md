# External services and dependencies

The service makes no outbound HTTP calls. Its only runtime external services are MongoDB and SMTP. AI chat replies come from `bloom-health-task-worker`, which shares the MongoDB database and calls the model provider (Groq); this service holds no model API key.

## Runtime services
| Service | Used for | Code | Configuration |
|---|---|---|---|
| MongoDB (Atlas) | All persistent data | `pkg/drivers/mongodb.go`, `pkg/infrastructure/mongorepo/` | `STORE`, `MONGODB_URI` (secret), `MONGODB_DB` |
| SMTP server | OTP emails | `pkg/infrastructure/transport/smtp/` | `MAILER`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER` and `SMTP_PASS` (secret), `MAIL_FROM` |

### MongoDB
- **Writes:** no multi-document transactions. Writes are single-document compare-and-swap on `version`, so any Atlas tier works.
- **Startup:** the service connects and pings, then runs `EnsureIndexes` in the background, so a cold start on Vercel doesn't make its first request wait for every index check. It logs `mongo connected` and `indexes ensured` with their `ms`, which shows cold-start cost in Vercel's logs. It needs `createIndex` rights, or see `ddl/mongodb/indexes.js`. On a brand-new database the unique indexes appear a moment after startup; provision with `indexes.js` first if that matters.
- **Access:** use a dedicated `readWrite` user on this database only, and restrict Atlas Network Access to the host's egress IPs.
- **Automatic expiry (TTL indexes):** `appliedOperations` after 60 days, plus `otpChallenges` and `refreshSessions` at `expiresAt`.
- **Collections:** `users`, `sharedSpaces`, `memberships`, `habits`, `dailyHabitLogs`, `workoutSessions`, `settings`, `periods`, `appliedOperations`, `otpChallenges`, `refreshSessions`, `counters`, `chatSessions`, `chatMessages` (the last two are shared with bloom-health-task-worker, see `docs/chat-contract.md`).
- **Names:** the database (`bubu_bloom`) keeps an older project name. Renaming it isn't cosmetic: it would point the service at a different database. The JWT issuer is `bloom-health`; changing it invalidates current access tokens, but not refresh tokens.
- **Testing:** see `_shared/testing-limits.md`.

### SMTP
- `MAILER=smtp` is required in PROD, and `MAILER=console` prints codes to the log for local use.
- No provider is fixed in the code. A provider API would be a new `smtp.IMailer` implementation.
- `prod-config.json` has a `no-reply@localhost` sender, so set `MAIL_FROM` in production.
- `net/smtp` is used, and it doesn't take the request's context, so there is no per-send timeout from the caller.

## Sibling services
| Service | Notes |
|---|---|
| `bloom-health-task-worker` | Separate repo. Claims pending `chatMessages`, calls Groq (`openai/gpt-oss-120b`, free tier) with `GROQ_API_KEY`, and settles them. Must use the same database and the same `chat.models` catalogue. Contract: `docs/chat-contract.md`. Runs on a Google Cloud e2-micro VM under systemd, deployed from its `v*` tags (its `deploy/gcp/README.md`). |

## Clients
| Client | Notes |
|---|---|
| `bloom-health-app` (Expo) | Reaches the API at `EXPO_PUBLIC_API_BASE_URL`. Never talks to MongoDB. Shares `docs/api-contract.md`, and keeps a copy of `testdata/scoring.json` at `src/lib/__fixtures__/scoring.json`. |

The app's fallback URL is `http://localhost:8080`, but this service defaults to port 3000, so set the variable for local runs.

## Build and delivery
| Tool | Notes |
|---|---|
| GitHub Actions (`.github/workflows/ci.yml`, `gitleaks.yml`) | vet, race tests, build and govulncheck, plus a full-history secret scan, on push to `main` and on pull requests. Actions are pinned to full SHAs |
| Dependabot (`.github/dependabot.yml`) | Weekly updates for Go modules, Actions and the Docker base images |
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
- **Account purge:** not in this service. bloom-health-task-worker erases accounts whose 15-day deletion grace has ended, daily at 10:00 IST, using the same erase order (`docs/account-deletion-contract.md`). `authService.eraseIfOverdue` erases an overdue account at its next sign-in before that run.
