package publicrouter

import (
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/assets"
)

// The page has inline styles and no scripts, so its policy only adds 'unsafe-inline' styles to the global one.
const privacyCSP = "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'"

var privacyPage = sync.OnceValue(func() string { return renderPolicyPage(assets.PrivacyPolicy) })

// PrivacyPolicy is GET /privacy: the public privacy policy page that Google Play and the App Store link to.
func PrivacyPolicy(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Security-Policy", privacyCSP)
	h.Set("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(privacyPage()))
}

var emailRe = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

// inline escapes a line and links email addresses. It runs after escaping, and addresses contain no escaped characters.
func inline(s string) string {
	return emailRe.ReplaceAllStringFunc(html.EscapeString(s), func(m string) string {
		return `<a href="mailto:` + m + `">` + m + `</a>`
	})
}

// renderPolicyPage turns the policy's small Markdown subset (# and ## headings, - bullets, an _italic_ line, paragraphs) into a page.
func renderPolicyPage(md string) string {
	var body strings.Builder
	title := "Privacy policy"
	inList := false
	closeList := func() {
		if inList {
			body.WriteString("</ul>\n")
			inList = false
		}
	}
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			closeList()
		case strings.HasPrefix(line, "# "):
			closeList()
			title = strings.TrimPrefix(line, "# ")
			body.WriteString("<h1>" + inline(title) + "</h1>\n")
		case strings.HasPrefix(line, "## "):
			closeList()
			body.WriteString("<h2>" + inline(strings.TrimPrefix(line, "## ")) + "</h2>\n")
		case strings.HasPrefix(line, "- "):
			if !inList {
				body.WriteString("<ul>\n")
				inList = true
			}
			body.WriteString("<li>" + inline(strings.TrimPrefix(line, "- ")) + "</li>\n")
		case len(line) > 2 && strings.HasPrefix(line, "_") && strings.HasSuffix(line, "_"):
			closeList()
			body.WriteString(`<p class="muted"><em>` + inline(line[1:len(line)-1]) + "</em></p>\n")
		default:
			closeList()
			body.WriteString("<p>" + inline(line) + "</p>\n")
		}
	}
	closeList()
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>
:root { color-scheme: light dark; --bg: #faf8ff; --text: #1f1b2e; --muted: #6b6580; --accent: #6c45ef; }
@media (prefers-color-scheme: dark) { :root { --bg: #15121f; --text: #ece9f5; --muted: #a39dba; --accent: #a58bff; } }
body { margin: 0; background: var(--bg); color: var(--text); font: 16px/1.6 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; }
main { max-width: 720px; margin: 0 auto; padding: 32px 16px 64px; }
h1 { font-size: 1.8rem; line-height: 1.25; margin: 0 0 4px; }
h2 { font-size: 1.15rem; margin: 28px 0 8px; color: var(--accent); }
ul { padding-left: 20px; }
li { margin: 6px 0; }
a { color: var(--accent); }
.muted { color: var(--muted); margin-top: 0; }
</style>
</head>
<body>
<main>
` + body.String() + `</main>
</body>
</html>
`
}
