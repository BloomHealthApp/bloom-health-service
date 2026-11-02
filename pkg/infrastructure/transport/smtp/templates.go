package smtp

import (
	"fmt"
	"html"
)

// Palette of the email design.
const (
	colorInk    = "#1F1B2E"
	colorBody   = "#51506A"
	colorPanel  = "#F3F0FC"
	colorBorder = "#E6E6EC"
	colorMuted  = "#7A7A8C"
)

// Backgrounds use bgcolor first, then a plain gradient, then a layered one: a client drops what it can't render
// and keeps the last declaration it understands. All of them keep the white header text readable.
const (
	bgFallback = "#6D4AF0"
	bgSimple   = "linear-gradient(160deg,#8A4DF5 0%,#3F6BF0 30%,#F06BB8 65%,#F9A23C 100%)"
	bgLayered  = "linear-gradient(to bottom left,rgba(56,166,245,.95) 0%,rgba(56,166,245,0) 45%)," +
		"linear-gradient(to bottom,rgba(84,62,242,.97) 0%,rgba(138,77,245,.75) 30%,rgba(138,77,245,0) 62%)," +
		"linear-gradient(to right,#F562B8 0%,#F98BA8 50%,#F9A23C 100%)"
)

// OTPKind selects the wording of the code email.
type OTPKind int

// Email kinds: a code that finishes creating an account, signs in to an existing one, or confirms a phone number.
const (
	OTPSignIn OTPKind = iota
	OTPSignUp
	OTPPhoneChange
)

// OTPEmail renders the one-time-code email. name is only used for OTPSignUp ("Welcome, <name>!") and is HTML-escaped.
func OTPEmail(kind OTPKind, name, code string, minutes int) (subject, text, htmlBody string) {
	var greeting, intro string
	if kind == OTPPhoneChange {
		subject = "Confirm your Bloom Health phone number"
		greeting = "Hello!"
		intro = "Use this code to add this phone number to your Bloom Health account"
	} else if kind == OTPSignUp {
		subject = "Your Bloom Health verification code"
		greeting = fmt.Sprintf("Welcome, %s!", name)
		intro = "Use this code to finish creating your Bloom Health account"
	} else {
		subject = "Your Bloom Health sign-in code"
		greeting = "Welcome back!"
		intro = "Use this code to sign in to Bloom Health"
	}
	text = fmt.Sprintf("Bloom Health\n\n%s\n%s: %s\n\nIt expires in %d minutes. "+
		"If you didn't ask for this, you can safely ignore this email.\n", greeting, intro, code, minutes)

	content := fmt.Sprintf(`<tr><td style="padding:0 32px;font-family:Arial,Helvetica,sans-serif;">
          <div style="font-size:22px;line-height:30px;font-weight:700;color:%[1]s;">%[2]s</div>
          <div style="font-size:15px;line-height:23px;color:%[3]s;padding-top:8px;">%[4]s:</div>
        </td></tr>
        <tr><td style="padding:20px 32px;">
          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"><tr>
            <td align="center" bgcolor="%[5]s" style="background-color:%[5]s;border-radius:8px;padding:16px 8px;font-family:Arial,Helvetica,sans-serif;font-size:32px;line-height:40px;font-weight:700;letter-spacing:6px;color:%[1]s;">%[6]s</td>
          </tr></table>
        </td></tr>
        <tr><td style="padding:0 32px 24px 32px;font-family:Arial,Helvetica,sans-serif;font-size:15px;line-height:23px;color:%[3]s;">
          This code expires in <strong>%[7]d minutes</strong>. Enter it in the app; there&rsquo;s nothing to click.
        </td></tr>`, colorInk, html.EscapeString(greeting), colorBody, html.EscapeString(intro), colorPanel, html.EscapeString(code), minutes)
	return subject, text, layout(html.EscapeString(subject), content)
}

// layout wraps a template's rows in the shared Bloom Health frame: gradient page, logo header, white card, safety
// note and footer. Every email should use it. Tables and inline CSS only, because mail clients strip <style> blocks
// and ignore modern layout. No images, because clients list inline ones as attachments and block remote ones, so
// the logo is an emoji. title and content must already be HTML-escaped.
func layout(title, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" style="height:100%%;">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="color-scheme" content="light">
<title>%[1]s</title>
</head>
<body style="margin:0;padding:0;height:100%%;background-color:%[2]s;">
<table role="presentation" width="100%%" height="100%%" cellpadding="0" cellspacing="0" border="0" bgcolor="%[2]s" style="height:100%%;background-color:%[2]s;background-image:%[3]s;background-image:%[4]s;">
  <tr><td align="center" valign="top" style="padding:28px 12px 32px 12px;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
      <td style="padding-right:12px;font-size:40px;line-height:48px;">&#129419;</td>
      <td align="left" style="font-family:Arial,Helvetica,sans-serif;color:#FFFFFF;">
        <div style="font-size:24px;line-height:30px;font-weight:700;">Bloom Health</div>
        <div style="font-size:14px;line-height:20px;">small steps, kind streaks</div>
      </td>
    </tr></table>
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" bgcolor="#FFFFFF" style="max-width:520px;margin-top:20px;background-color:#FFFFFF;border-radius:12px;">
      <tr><td style="height:28px;font-size:1px;line-height:1px;">&nbsp;</td></tr>
      %[5]s
      <tr><td style="padding:0 32px;"><div style="border-top:1px solid %[6]s;font-size:1px;line-height:1px;">&nbsp;</div></td></tr>
      <tr><td style="padding:20px 32px 28px 32px;font-family:Arial,Helvetica,sans-serif;font-size:13px;line-height:20px;color:%[7]s;">
        Didn&rsquo;t ask for this? You can safely ignore this email. Nobody can sign in without the code.<br><br>
        Bloom Health is a personal wellness tracker, not medical advice.
      </td></tr>
    </table>
  </td></tr>
</table>
</body>
</html>`, title, bgFallback, bgSimple, bgLayered, content, colorBorder, colorMuted)
}
