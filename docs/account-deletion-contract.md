# Account deletion contract

Shared by **bloom-health-service** (schedules, locks, restores and erases) and **bloom-health-task-worker** (triggers the daily erase). Keep this file identical in both repos.

## Flow
1. `DELETE /v1/me` (service) sets `users.deletionRequestedAt` and `users.deleteAfter` (+15 days), revokes the user's refresh sessions and emails the date. The account is locked: every authenticated route except `GET /v1/me`, `DELETE /v1/me` and `POST /v1/me/restore` answers `403 account_pending_deletion`, also for viewers of that owner's space.
2. `POST /v1/me/restore` (service) clears both fields before `deleteAfter`.
3. **Every day at 10:00 Asia/Kolkata the worker calls the service:**

   `POST https://<service>/internal/accounts/purge-deleted` with `Authorization: Bearer <INTERNAL_API_SECRET>` and no body.

   - **200** `{"erased": <n>}`: every account whose `deleteAfter` has passed is now erased.
   - **401**: wrong or missing secret.
   - **404**: the service has no `INTERNAL_API_SECRET` set.
   - **5xx or network error**: the worker retries a few times with backoff. The call is idempotent, so the next day's run also catches up.
4. The service also erases an overdue account at its next sign-in, so nobody can restore after the date even before the 10:00 run.

## Erase (service, `eraseAccount`)
For an owner, the whole space first: habits, daily logs, workouts, settings, periods, sync history and every membership. Then the user: chats, refresh sessions, pending codes, memberships and the user document, last. Each step is idempotent.

## Secret
`INTERNAL_API_SECRET` is a long random string, set in the service's Vercel env and in the worker's `/etc/bloom-worker/env`. Never commit or log it. Rotate it in both places together.
