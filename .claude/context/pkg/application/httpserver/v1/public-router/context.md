# pkg/application/httpserver/v1/public-router/

## Purpose
The `/v1` handlers. `docs/api-contract.md` is the source of truth for request and response shapes. The notes here add what the contract doesn't say: wiring, limits and code-level behavior.

## Rules
One file per endpoint, kebab-case after the action (`create-habit.go`), holding one `HandleXxx` func. `router.go` only registers routes; binders shared by several handlers live in `bind-helpers.go`. Each API area has one note here (`habits.md` covers the four habit handlers).

Every handler has the same shape, with no logic in handlers:

1. `ctx := c.Request.Context()`
2. Bind with `utils.BindStrictJSON` (rejects unknown fields) and fill the DTO.
3. `req.Validate()`
4. `di.AppLogic.GetXxxService().Method(ctx, actor, &req)`, with the actor from `utils.GetActor(ctx)`.
5. `c.JSON(status, resp)` or `utils.RespondError(c, err)`.

When adding or changing a route, update `router.go`, the contract, the table below and the API's note.

## All routes
Access: public = no token; both = any member; owner = `RequireOwner`.

| Route | Access | Service | Note |
|---|---|---|---|
| `GET /health`, `GET /v1/health` | public | Health | [health.md](health.md) |
| `GET /privacy` | public | — | [privacy.md](privacy.md) |
| `GET /delete-account` | public | — | [privacy.md](privacy.md) |
| `POST /v1/auth/signup/request` | public | Auth | [auth.md](auth.md) |
| `POST /v1/auth/signin/request` | public | Auth | [auth.md](auth.md) |
| `POST /v1/auth/otp/verify` | public | Auth | [auth.md](auth.md) |
| `POST /v1/auth/google` | public | Auth | [auth.md](auth.md) |
| `POST /v1/auth/refresh` | public | Auth | [auth.md](auth.md) |
| `POST /v1/auth/logout` | public | Auth | [auth.md](auth.md) |
| `GET /v1/me` | both | User | [me.md](me.md) |
| `PATCH /v1/me` | both | User | [me.md](me.md) |
| `DELETE /v1/me` | both | User | [me.md](me.md) |
| `POST /v1/me/restore` | both | User | [me.md](me.md) |
| `POST /internal/accounts/purge-deleted` | `INTERNAL_API_SECRET` | User | [me.md](me.md); called by bloom-health-task-worker daily at 10:00 IST |
| `GET /v1/settings` | both | Settings | [settings.md](settings.md) |
| `PATCH /v1/settings` | owner | Settings | [settings.md](settings.md) |
| `GET /v1/habits` | both | Habit | [habits.md](habits.md) |
| `POST /v1/habits` | owner | Habit | [habits.md](habits.md) |
| `PATCH /v1/habits/:id` | owner | Habit | [habits.md](habits.md) |
| `DELETE /v1/habits/:id` | owner | Habit | [habits.md](habits.md) |
| `GET /v1/logs` | both | DailyLog | [logs.md](logs.md) |
| `PUT /v1/logs/:habitId/:localDate` | owner | DailyLog | [logs.md](logs.md) |
| `GET /v1/workouts` | both | Workout | [workouts.md](workouts.md) |
| `POST /v1/workouts` | owner | Workout | [workouts.md](workouts.md) |
| `PATCH /v1/workouts/:id` | owner | Workout | [workouts.md](workouts.md) |
| `DELETE /v1/workouts/:id` | owner | Workout | [workouts.md](workouts.md) |
| `GET /v1/periods` | owner | Period | [periods.md](periods.md) |
| `POST /v1/periods` | owner | Period | [periods.md](periods.md) |
| `PATCH /v1/periods/:id` | owner | Period | [periods.md](periods.md) |
| `DELETE /v1/periods/:id` | owner | Period | [periods.md](periods.md) |
| `GET /v1/stats/summary` | both | Stats | [stats.md](stats.md) |
| `POST /v1/sync/push` | owner | Sync | [sync.md](sync.md) |
| `GET /v1/sync/pull` | both | Sync | [sync.md](sync.md) |
| `GET /v1/chat/models` | both | Chat | [chat.md](chat.md) |
| `POST /v1/chat/sessions` | both | Chat | [chat.md](chat.md) |
| `GET /v1/chat/sessions` | both | Chat | [chat.md](chat.md) |
| `GET /v1/chat/sessions/:sid` | both | Chat | [chat.md](chat.md) |
| `DELETE /v1/chat/sessions/:sid` | both | Chat | [chat.md](chat.md) |
| `GET /v1/chat/sessions/:sid/messages` | both | Chat | [chat.md](chat.md) |
| `POST /v1/chat/sessions/:sid/messages` | both | Chat | [chat.md](chat.md) |
