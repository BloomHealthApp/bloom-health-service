# pkg/infrastructure/transport/smtp/

## Purpose
`IMailer` with three implementations: smtp, console (dev) and capture (tests).

## Gotchas
The console mailer prints OTP codes, so PROD refuses it (see `config.Store.Validate`).
New emails render through `layout` in `templates.go`, so they share the design. It is plain HTML and inline CSS: a gradient page, a butterfly emoji as the logo, and a simple white card. It has no images, because inline ones showed as attachments in Gmail and remote ones can be blocked. Every gradient needs a solid fallback that keeps the text readable. Keep hidden preheader text out too, as hidden text looks like spam. Mail clients run no scripts, so there is no copy button.
`net/smtp` adds no headers, so `buildMessage` sets `Date` and `Message-ID`. Gmail filed the redesigned OTP email as spam when sent from a free Gmail address ("similar to messages identified as spam"); a verified domain with SPF and DKIM is the reliable fix for delivery.
