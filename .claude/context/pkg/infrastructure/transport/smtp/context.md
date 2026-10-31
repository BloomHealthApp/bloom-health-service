# pkg/infrastructure/transport/smtp/

## Purpose
`IMailer` with three implementations: smtp, console (dev) and capture (tests).

## Gotchas
The console mailer prints OTP codes, so PROD refuses it (see `config.Store.Validate`).
