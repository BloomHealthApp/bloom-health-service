# Account deletion contract

Shared by **bloom-health-service** (schedules, locks and restores) and **bloom-health-task-worker** (erases). Keep this file identical in both repos.

## Fields on `users`
| Field | Type | Written by | Meaning |
|---|---|---|---|
| `deletionRequestedAt` | date, absent when not requested | service | when `DELETE /v1/me` was called |
| `deleteAfter` | date, absent when not requested | service | `deletionRequestedAt` + 15 days; the account is erased once this has passed |

`users.deleteAfter` has a sparse index (created by the service). `POST /v1/me/restore` removes both fields before `deleteAfter`.

## While pending (service)
The account is locked: every authenticated route except `GET /v1/me`, `DELETE /v1/me` and `POST /v1/me/restore` answers `403 account_pending_deletion`, also for viewers of that owner's space. Refresh sessions are revoked when deletion is requested.

## Erase (worker daily at 10:00 Asia/Kolkata; service at sign-in for an overdue account)
Due: `deleteAfter <= now`. For each due user, in this order. Every step is a `deleteMany` and is idempotent, so a failed erase is retried by the next run.

1. **Space, only if the user is its owner.** Look up the user's membership (`memberships {userId}`). If its `role` is `owner`, delete by its `sharedSpaceId`:
   - `habits`, `dailyHabitLogs`, `workoutSessions`, `settings`, `periods`: `{spaceId}`
   - `appliedOperations`: `{spaceId}`
   - `counters`: `{_id: spaceId}`
   - `memberships`: `{sharedSpaceId}` (every viewer loses access)
   - `sharedSpaces`: `{_id: spaceId}`
2. **User:**
   - `chatMessages`: `{userId}`
   - `chatSessions`: `{userId}`
   - `refreshSessions`: `{userId}`
   - `otpChallenges`: `{email: <lower-cased user email>}`
   - `memberships`: `{userId}`
   - `users`: `{_id}`, last, so a partial failure leaves the user to retry

A new collection holding user or space data must be added here, to the service's `IAccountRepo`, and to the worker's `IAccountRepo`.

Never log emails, names or any erased content; log user ids and counts only.
