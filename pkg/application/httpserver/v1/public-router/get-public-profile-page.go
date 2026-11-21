package publicrouter

import (
	"html"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
)

const profilePageCSP = "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'"

// HandleGetPublicProfilePage is GET /u/:code, the page a shared profile link opens when the app isn't installed
// (with the app, Android App Links open it in the app instead). It is reachable without signing in, so it shows only
// a first name and an initial, and an unknown, reset or deleted link gets one identical answer.
func HandleGetPublicProfilePage(c *gin.Context) {
	cfg := di.AppLogic.GetConfig()
	code := c.Param("code")
	card, ok := di.AppLogic.GetConnectionService().PublicCard(c.Request.Context(), code)
	h := c.Writer.Header()
	h.Set("Content-Security-Policy", profilePageCSP)
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Cache-Control", "no-store")
	status := http.StatusOK
	if !ok {
		status = http.StatusNotFound
		card = nil
	}
	c.Data(status, "text/html; charset=utf-8", []byte(renderProfilePage(card, "bloomhealth://u/"+code, cfg.PlayStoreURL)))
}

func renderProfilePage(card *bo.PublicCard, appURL, storeURL string) string {
	title, body := "This link isn't active", `<p class="muted">Ask for a new link to their Bloom profile.</p>`
	actions := `<a class="btn" href="` + html.EscapeString(storeURL) + `">Get Bloom</a>`
	if card != nil {
		name := html.EscapeString(card.FirstName)
		title = name + " on Bloom"
		body = `<div class="avatar" aria-hidden="true">` + html.EscapeString(card.Initial) + `</div>
<h1>` + name + `</h1>
<p class="muted">` + name + ` would like to be friends on Bloom. Open the app to see their profile and add them.</p>`
		actions = `<a class="btn" href="` + html.EscapeString(appURL) + `">Open in Bloom</a>
<a class="btn secondary" href="` + html.EscapeString(storeURL) + `">Get Bloom</a>`
	}
	if card == nil {
		body = `<h1>` + html.EscapeString(title) + `</h1>` + body
	}
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>` + html.EscapeString(title) + `</title>
<style>
:root { color-scheme: light dark; --bg: #faf8ff; --text: #1f1b2e; --muted: #6b6580; --accent: #6c45ef; --on: #ffffff; --tint: #ece6ff; }
@media (prefers-color-scheme: dark) { :root { --bg: #15121f; --text: #ece9f5; --muted: #a39dba; --accent: #a58bff; --on: #15121f; --tint: #2a2440; } }
body { margin: 0; background: var(--bg); color: var(--text); font: 16px/1.5 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; }
main { max-width: 420px; margin: 0 auto; padding: 56px 16px; text-align: center; }
.avatar { width: 88px; height: 88px; margin: 0 auto 16px; border-radius: 50%; background: var(--tint); color: var(--accent); font-size: 40px; font-weight: 700; line-height: 88px; }
h1 { font-size: 1.6rem; margin: 0 0 8px; }
.muted { color: var(--muted); margin: 0 0 28px; }
.btn { display: block; margin: 0 0 12px; padding: 14px; border-radius: 999px; background: var(--accent); color: var(--on); font-weight: 600; text-decoration: none; }
.btn.secondary { background: var(--tint); color: var(--accent); }
</style>
</head>
<body>
<main>
` + body + `
` + actions + `
</main>
</body>
</html>
`
}
