package smtp

import (
	"fmt"
	"html"
	"strings"
)

// Palette mirrors the app's default "Cotton Candy" theme.
const (
	colorBlush = "#F8D7E8"
	colorRose  = "#D65B98"
	colorCream = "#FFF9F5"
	colorPlum  = "#4A3A4F"
	colorMuted = "#8A7790"
)

// OTPKind selects the wording of the code email.
type OTPKind int

// Email kinds: a code that finishes creating an account, or signs in to an existing one.
const (
	OTPSignIn OTPKind = iota
	OTPSignUp
)

// OTPEmail renders the one-time-code email. Layout uses tables and inline CSS only, because mail clients
// strip <style> blocks and ignore modern layout; no remote images are used so nothing is blocked.
// name is only used for OTPSignUp ("Welcome, <name>!") and is HTML-escaped.
func OTPEmail(kind OTPKind, name, code string, minutes int) (subject, text, htmlBody string) {
	var greetText, greetHTML, intro string
	if kind == OTPSignUp {
		subject = "Your Bubu Bloom verification code"
		greetText = fmt.Sprintf("Welcome, %s!", name)
		greetHTML = fmt.Sprintf("Welcome, %s! &#127800;", html.EscapeString(name))
		intro = "Use this code to finish creating your Bubu Bloom account:"
	} else {
		subject = "Your Bubu Bloom sign-in code"
		greetText = "Welcome back!"
		greetHTML = "Welcome back! &#127800;"
		intro = "Use this code to sign in to Bubu Bloom:"
	}
	text = fmt.Sprintf("Bubu Bloom\n\n%s\n%s %s\n\nIt expires in %d minutes. "+
		"If you didn't ask for this, you can safely ignore this email.\n", greetText, intro, code, minutes)

	spaced := strings.Join(strings.Split(html.EscapeString(code), ""), "&#8202;")
	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="color-scheme" content="light">
<title>%[1]s</title>
</head>
<body style="margin:0;padding:0;background:%[2]s;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:%[2]s;">Your Bubu Bloom code is %[4]s. It expires in %[5]d minutes.</div>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:%[2]s;">
  <tr><td align="center" style="padding:32px 16px;">
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:480px;background:#FFFFFF;border-radius:24px;border:1px solid %[2]s;">
      <tr><td align="center" style="background:%[2]s;border-radius:24px 24px 0 0;padding:28px 24px 20px 24px;">
        <div style="font-size:44px;line-height:48px;">&#128059;</div>
        <div style="font-family:'Trebuchet MS',Verdana,Arial,sans-serif;font-size:24px;line-height:30px;font-weight:700;color:%[3]s;padding-top:6px;">Bubu Bloom</div>
        <div style="font-family:Arial,Helvetica,sans-serif;font-size:13px;line-height:18px;color:%[6]s;padding-top:2px;">small steps, kind streaks</div>
      </td></tr>
      <tr><td align="center" style="padding:28px 28px 8px 28px;font-family:Arial,Helvetica,sans-serif;">
        <div style="font-size:18px;line-height:26px;font-weight:700;color:%[7]s;">%[10]s</div>
        <div style="font-size:15px;line-height:22px;color:%[7]s;padding-top:8px;">%[11]s</div>
      </td></tr>
      <tr><td align="center" style="padding:16px 28px;">
        <table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
          <td align="center" style="background:%[8]s;border:2px dashed %[3]s;border-radius:16px;padding:16px 28px;font-family:'Courier New',Courier,monospace;font-size:36px;line-height:42px;font-weight:700;letter-spacing:8px;color:%[3]s;">%[9]s</td>
        </tr></table>
      </td></tr>
      <tr><td align="center" style="padding:4px 28px 24px 28px;font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:21px;color:%[6]s;">
        This code expires in <strong style="color:%[7]s;">%[5]d minutes</strong>.<br>
        Enter it in the app &mdash; no need to click anything.
      </td></tr>
      <tr><td style="padding:0 28px;"><div style="border-top:1px solid %[2]s;line-height:1px;font-size:1px;">&nbsp;</div></td></tr>
      <tr><td align="center" style="padding:18px 28px 28px 28px;font-family:Arial,Helvetica,sans-serif;font-size:12px;line-height:18px;color:%[6]s;">
        Didn&rsquo;t ask for this? You can safely ignore this email &mdash; nobody can sign in without the code.<br>
        <span style="color:%[3]s;">&#10084;</span> Bubu Bloom is a personal wellness tracker, not medical advice.
      </td></tr>
    </table>
  </td></tr>
</table>
</body>
</html>`, html.EscapeString(subject), colorBlush, colorRose, html.EscapeString(code), minutes, colorMuted, colorPlum, colorCream, spaced, greetHTML, html.EscapeString(intro))
	return subject, text, htmlBody
}
