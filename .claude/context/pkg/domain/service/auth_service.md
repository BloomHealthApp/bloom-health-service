# IAuthService (`auth_service.go`)

## Purpose
Passwordless sign-up and sign-in by emailed code, JWT access tokens, rotating refresh tokens, and resolving the actor on each request.

## Rules
- `RequestSignIn` always returns a challenge. Unknown identifiers and internal failures return a random decoy id, and the failure is only logged.
- Every `VerifyOTP` failure returns the same 401 `invalid or expired code`.
- `Authenticate` takes the role from the database, never from the token. It only accepts HS256 with the configured issuer and an expiry.
- Compare OTP hashes with `subtle.ConstantTimeCompare`.

## Why
- The OTP HMAC includes the challenge id, so a stored hash can't be reused for another challenge.
- Refresh tokens are stored as SHA-256. Reusing a rotated token revokes the whole family, because reuse suggests theft.

## Gotchas
- Sending a new code deletes the previous challenge first, so only the newest code works. If the email then fails to send, the old code is already gone.
- The cooldown (`auth.otpResendSeconds`) is per email. During it, sign-in returns the existing challenge id with no new mail, but only if that challenge is a sign-in one. Sign-up returns 429.
- `VerifyOTP` deletes the challenge before provisioning, so a provisioning failure means the user must request a new code.
- `AccessSecret` keys both JWTs and OTP hashes, so rotating it invalidates access tokens and pending codes. Refresh tokens survive because they use plain SHA-256.
- `mailer.Send` runs inside the request and the SMTP mailer ignores `ctx`, so a slow mail server slows the API call.

## Related
[provision_service.md](provision_service.md), `_shared/security-invariants.md`, [auth API](../../application/httpserver/v1/public-router/auth.md)
