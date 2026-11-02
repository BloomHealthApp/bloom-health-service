package smtp

import (
	"embed"
	"fmt"
	"html"
	"strings"
)

// Palette of the email design.
const (
	colorInk     = "#22134F"
	colorBody    = "#4A3F73"
	colorPink    = "#E0348C"
	colorPurple  = "#6D3FE8"
	colorPanel   = "#F3EAFB"
	colorDivider = "#EBDDF3"
	colorFooter  = "#5D5285"
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

//go:embed images/*.png
var imageFS embed.FS

// inlineImages maps each Content-ID the templates reference to its embedded file.
var inlineImages = []struct{ cid, file string }{
	{"bloom-logo", "images/logo.png"},
	{"bloom-hero", "images/hero.png"},
	{"bloom-clock", "images/clock.png"},
	{"bloom-shield", "images/shield.png"},
}

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
	var greetText, greetHTML, intro string
	if kind == OTPPhoneChange {
		subject = "Confirm your Bloom Health phone number"
		greetText = "Hello!"
		greetHTML = "Hello!"
		intro = "Use this code to add this phone number to your Bloom Health account"
	} else if kind == OTPSignUp {
		subject = "Your Bloom Health verification code"
		greetText = fmt.Sprintf("Welcome, %s!", name)
		greetHTML = fmt.Sprintf("Welcome, %s! &#127800;", html.EscapeString(name))
		intro = "Use this code to finish creating your Bloom Health account"
	} else {
		subject = "Your Bloom Health sign-in code"
		greetText = "Welcome back!"
		greetHTML = "Welcome back! &#127800;"
		intro = "Use this code to sign in to Bloom Health"
	}
	text = fmt.Sprintf("Bloom Health\n\n%s\n%s: %s\n\nIt expires in %d minutes. "+
		"If you didn't ask for this, you can safely ignore this email.\n", greetText, intro, code, minutes)

	content := fmt.Sprintf(`<tr><td align="center" style="padding:0 20px;font-family:Arial,Helvetica,sans-serif;">
          <div style="font-size:30px;line-height:38px;font-weight:700;color:%[1]s;">%[2]s</div>
          <div style="font-size:16px;line-height:24px;color:%[1]s;padding-top:6px;">%[3]s</div>
        </td></tr>
        <tr><td align="center" style="padding:20px 12px 18px 12px;">%[4]s</td></tr>
        <tr><td align="center" style="padding:0 20px 20px 20px;font-family:Arial,Helvetica,sans-serif;font-size:15px;line-height:23px;color:%[5]s;">
          <img src="cid:bloom-clock" width="18" height="18" alt="" style="vertical-align:-4px;border:0;">&nbsp; This code expires in <strong style="color:%[6]s;">%[7]d minutes</strong><br>
          Enter it in the app &mdash; no need to click anything.
        </td></tr>`, colorInk, greetHTML, html.EscapeString(intro), codeTiles(code), colorBody, colorPink, minutes)
	preheader := fmt.Sprintf("Your Bloom Health code is %s. It expires in %d minutes.", html.EscapeString(code), minutes)
	return subject, text, layout(html.EscapeString(subject), preheader, content)
}

// codeTiles draws one tile per character. The tiles are inline with no whitespace between them, so selecting
// the row copies an unbroken code. Mail clients run no scripts, so a working copy button isn't possible.
func codeTiles(code string) string {
	var tiles strings.Builder
	for _, r := range code {
		tiles.WriteString(`<span style="display:inline-block;width:36px;margin:0 2px;background:#FFFFFF;border-radius:9px;font-family:Arial,Helvetica,sans-serif;font-size:24px;line-height:46px;font-weight:700;text-align:center;color:` + colorInk + `;">` +
			html.EscapeString(string(r)) + `</span>`)
	}
	return `<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>` +
		`<td align="center" bgcolor="#E9DEFA" style="background-color:#E9DEFA;background-image:linear-gradient(90deg,#DCCFFA,#F3E6FA 60%,#FDE3CB);border:2px solid #B9A0EC;border-radius:40px;padding:8px 10px;white-space:nowrap;">` +
		tiles.String() + `</td></tr></table>`
}

// layout wraps a template's rows in the shared Bloom Health frame: gradient page, logo header, hero, safety
// note and footer. Every email should use it. Tables and inline CSS only, because mail clients strip <style>
// blocks and ignore modern layout. Images are inline attachments, so no remote resource is fetched.
// title, preheader and content must already be HTML-escaped.
func layout(title, preheader, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" style="height:100%%;">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="color-scheme" content="light">
<title>%[1]s</title>
</head>
<body style="margin:0;padding:0;height:100%%;background:%[3]s;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;">%[2]s</div>
<table role="presentation" width="100%%" height="100%%" cellpadding="0" cellspacing="0" border="0" bgcolor="%[3]s" style="height:100%%;background-color:%[3]s;background-image:%[4]s;background-image:%[5]s;">
  <tr><td align="center" valign="top" style="padding:28px 8px 32px 8px;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
      <td style="padding-right:14px;"><img src="cid:bloom-logo" width="64" height="51" alt="" style="display:block;border:0;"></td>
      <td align="left" style="font-family:Arial,Helvetica,sans-serif;color:#FFFFFF;">
        <div style="font-size:26px;line-height:32px;font-weight:700;">Bloom Health</div>
        <div style="font-size:14px;line-height:20px;">small steps, kind streaks</div>
      </td>
    </tr></table>
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" bgcolor="#FFFFFF" style="max-width:560px;margin-top:24px;background-color:#FFFFFF;background-image:linear-gradient(170deg,#F2F4FF 0%%,#FDF1FA 55%%,#FFF6EE 100%%);border-radius:28px;">
        <tr><td align="center" style="padding:24px 12px 14px 12px;"><img src="cid:bloom-hero" width="320" alt="" style="display:block;width:320px;max-width:100%%;height:auto;border:0;"></td></tr>
        %[6]s
        <tr><td style="padding:0 24px;"><table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"><tr>
          <td valign="middle"><div style="border-top:1px solid %[7]s;font-size:1px;line-height:1px;">&nbsp;</div></td>
          <td width="36" align="center" style="width:36px;font-size:20px;line-height:20px;color:%[8]s;">&#9829;&#65038;</td>
          <td valign="middle"><div style="border-top:1px solid %[7]s;font-size:1px;line-height:1px;">&nbsp;</div></td>
        </tr></table></td></tr>
        <tr><td style="padding:18px 16px 14px 16px;">
          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" bgcolor="%[9]s" style="background-color:%[9]s;border-radius:16px;"><tr>
            <td width="36" style="width:36px;padding:14px 0 14px 16px;"><img src="cid:bloom-shield" width="32" height="37" alt="" style="display:block;border:0;"></td>
            <td style="padding:14px 16px 14px 14px;font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:21px;color:%[10]s;"><strong>Didn&rsquo;t ask for this?</strong> You can safely ignore this email &mdash; nobody can sign in without the code.</td>
          </tr></table>
        </td></tr>
        <tr><td align="center" style="padding:0 20px 24px 20px;font-family:Arial,Helvetica,sans-serif;font-size:13px;line-height:19px;color:%[11]s;"><span style="color:%[8]s;">&#9829;&#65038;</span> Bloom Health is a personal wellness tracker, not medical advice.</td></tr>
    </table>
  </td></tr>
</table>
</body>
</html>`, title, preheader, bgFallback, bgSimple, bgLayered, content, colorDivider, colorPink, colorPanel, colorInk, colorFooter)
}
