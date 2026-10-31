# pkg/infrastructure/validation/

## Purpose
Strict allowlist validation for entities: habit, dailyLog, workout, settings, period.

## Rules
Field rules live here. Period rules that need sibling rows live in `syncService.checkPeriodRules`.

## Why
The allowlists also prevent mass-assignment of server-controlled fields.
