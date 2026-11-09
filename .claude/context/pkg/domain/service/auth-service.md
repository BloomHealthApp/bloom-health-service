# IAuthService (`auth-service.go`)

## Purpose
Passwordless sign-up and sign-in by emailed code or Google ID token, JWT access tokens, rotating refresh tokens, and resolving the actor on each request.

## Rules
- `RequestSignIn` always returns a challenge. Unknown identifiers and internal failures return a random decoy id, and the failure is only logged.
- A phone sign-in that finds no account logs `signin: no account for phone`, without the number. An email that finds none logs nothing.
- A `phone_change` challenge is only consumed by `ConfirmPhoneChange`, for the user whose email it was mailed to. `VerifyOTP` never issues tokens for it.
- Every `VerifyOTP` failure returns the same 401 `invalid or expired code`.
- `Authenticate` takes the role from the database, never from the token. It only accepts HS256 with the configured issuer and an expiry.
- Compare OTP hashes with `subtle.ConstantTimeCompare`.
- `Authenticate` rejects an account whose deletion date has passed, and sets `Actor.DeleteAfter` for a pending one (for a viewer, from the space owner via `spaceOwner`). Sign-in request and verify run `eraseIfOverdue` first, so an overdue account is erased and answers like an unknown one.
- **Review login (Google Play):** when `REVIEW_LOGIN_EMAIL`/`REVIEW_LOGIN_CODE` are set, a *sign-in* challenge for that email is hashed from the fixed code and not mailed (logged as `signin: review account, code not mailed`). Sign-up and phone-change codes are always random and mailed, so the account must first be created with a real emailed code, and an unknown review email still gets the usual decoy. Attempts, rate limits and the cooldown apply unchanged. The account is an ordinary owner with its own private space.

- **Google sign-in** (`SignInWithGoogle`): `drivers.IGoogleVerifier` checks the token. Lookup order is `ByGoogleSub`, then `ByEmail` (auto-link: sets `GoogleSub`), then `EnsureAccount` with no phone, avatar or gender (`IsNewUser`). Linking by email relies on Google's `email_verified`; the verifier refuses tokens without it. An email whose account is already linked to another Google id is 409 `already_registered`, never relinked. A unique-index clash (two first sign-ins at once) is looked up once more before it becomes a 409. `eraseIfOverdue` runs on the found account, as in `VerifyOTP`. A Google sign-in never mails a code, and the review login doesn't apply to it.

## Why
- The OTP HMAC includes the challenge id, so a stored hash can't be reused for another challenge.
- Refresh tokens are stored as SHA-256. Reusing a rotated token revokes the whole family, because reuse suggests theft.

- The verifier wraps `google.golang.org/api/idtoken`, which never checks `iss` and also accepts ES256 tokens signed with Google's IAP keys. `pkg/drivers/google-verifier.go` pins the issuer and the audience before the signature check. Keep that if the library is swapped.

## Gotchas
- Sending a new code deletes the previous challenge first, so only the newest code works. If the email then fails to send, the old code is already gone.
- The cooldown (`auth.otpResendSeconds`) is per email. During it, sign-in returns the existing challenge id with no new mail, but only if that challenge is a sign-in one. Sign-up returns 429.
- `VerifyOTP` deletes the challenge before provisioning, so a provisioning failure means the user must request a new code.
- `AccessSecret` keys both JWTs and OTP hashes, so rotating it invalidates access tokens and pending codes. Refresh tokens survive because they use plain SHA-256.
- `mailer.Send` runs inside the request and the SMTP mailer ignores `ctx`, so a slow mail server slows the API call.

## Related
[provision-service.md](provision-service.md), `_shared/security-invariants.md`, [auth API](../../application/httpserver/v1/public-router/auth.md)
