# IProvisionService (`provision_service.go`)

## Purpose
Creates users, spaces, memberships, default habits and settings.

- `EnsureAccount`: used by `VerifyOTP` for sign-up, and creates a private space with the user as owner.
- `Provision`: creates an owner and viewer sharing one space. Only tests call it today, and no endpoint exposes it. It is the only way to create a viewer until invitations exist.

## Rules
- Default habit ids are stable slugs (`default-wakeup`, ...), listed in `defaultHabits`. They must match the contract's list.
- Defaults go through `validation.Merge` like any other entity, but are written straight to the repo, not through `ISyncService.Apply`.

## Gotchas
- There are no transactions. `EnsureAccount` returns early once a membership exists, so if seeding failed after that, a repeat call doesn't repair the missing habits. `ISettingsService.Get` still returns default settings in that case.
- `EnsureAccount` falls back to UTC for an invalid timezone, and caps names at 40 characters.
- A new user's timezone is `DEFAULT_TIMEZONE` until the app reports the device's timezone through `PATCH /me`.
