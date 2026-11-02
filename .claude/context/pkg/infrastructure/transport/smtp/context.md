# pkg/infrastructure/transport/smtp/

## Purpose
`IMailer` with three implementations: smtp, console (dev) and capture (tests).

## Gotchas
The console mailer prints OTP codes, so PROD refuses it (see `config.Store.Validate`).
New emails render through `layout` in `templates.go`, so they share the design. Its images are PNGs in `images/`, embedded in the binary and sent as inline attachments (`cid:`): `buildMessage` wraps the HTML in `multipart/related` when it references one. Gmail blocks data URIs and inline SVG, and remote images can be blocked, so don't switch to those.
Mail clients run no scripts and many drop gradients, so there's no copy button (selecting the tiles copies an unbroken code) and every gradient needs a solid fallback that keeps the text readable.
