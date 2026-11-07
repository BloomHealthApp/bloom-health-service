package publicrouter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderPolicyPage(t *testing.T) {
	page := renderPolicyPage("# Title <x>\n\n_Updated today_\n\nIntro & more.\n\n## Part\n\n- one\n- two\n\nMail a@b.io now.\n")
	require.Contains(t, page, "<title>Title &lt;x&gt;</title>")
	require.Contains(t, page, `<p class="muted"><em>Updated today</em></p>`)
	require.Contains(t, page, "<p>Intro &amp; more.</p>")
	require.Contains(t, page, "<h2>Part</h2>\n<ul>\n<li>one</li>\n<li>two</li>\n</ul>\n")
	require.Contains(t, page, `<p>Mail <a href="mailto:a@b.io">a@b.io</a> now.</p>`)
}

func TestRenderPolicyPageLinksURLs(t *testing.T) {
	page := renderPolicyPage("Read https://example.com/privacy. Mail a@b.io.\n")
	require.Contains(t, page, `<a href="https://example.com/privacy">https://example.com/privacy</a>.`)
	require.Contains(t, page, `<a href="mailto:a@b.io">a@b.io</a>`)
}
