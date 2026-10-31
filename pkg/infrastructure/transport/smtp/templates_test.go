package smtp

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOTPEmailSignInContent(t *testing.T) {
	subject, text, html := OTPEmail(OTPSignIn, "", "123456", 10)
	require.Equal(t, "Your Bloom Health sign-in code", subject)
	require.Contains(t, text, "123456")
	require.Contains(t, text, "10 minutes")
	require.Contains(t, text, "Welcome back")
	require.Contains(t, html, "<!DOCTYPE html>")
	require.Contains(t, html, "Bloom Health")
	require.Contains(t, html, "Welcome back")
	require.Contains(t, html, "10 minutes")
	require.Contains(t, html, "&#8202;", "digits are spaced for readability")
	require.NotContains(t, html, "<script")
	require.NotContains(t, html, "http://", "no remote resources")
	require.NotContains(t, html, "%!", "no formatting errors")
}

func TestOTPEmailSignUpGreetsByName(t *testing.T) {
	subject, text, html := OTPEmail(OTPSignUp, "Anu", "123456", 10)
	require.Equal(t, "Your Bloom Health verification code", subject)
	require.Contains(t, text, "Welcome, Anu!")
	require.Contains(t, html, "Welcome, Anu!")
	require.NotContains(t, html, "Welcome back")
	require.NotContains(t, html, "%!", "no formatting errors")
}

func TestOTPEmailEscapesNameAndCode(t *testing.T) {
	_, _, html := OTPEmail(OTPSignUp, `<script>alert("x")</script>&`, "123456", 5)
	require.NotContains(t, html, "<script>")
	require.Contains(t, html, "&lt;script&gt;")
	require.Contains(t, html, "&amp;")
	_, _, html = OTPEmail(OTPSignIn, "", `<b>"&`, 5)
	require.NotContains(t, html, "<b>")
	require.Contains(t, html, "&lt;b&gt;")
}

func TestBuildMessageIsValidMultipart(t *testing.T) {
	from, _ := formatFrom("no-reply@example.com")
	require.Equal(t, `"Bloom Health" <no-reply@example.com>`, from)
	raw, err := buildMessage(from, "user@example.com", "Your Bloom Health sign-in code", "plain 123456", "<p>html 123456 🌸</p>")
	require.NoError(t, err)

	m, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)
	require.Equal(t, "user@example.com", m.Header.Get("To"))
	mt, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/alternative", mt)

	mr := multipart.NewReader(m.Body, params["boundary"])
	var types, bodies []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		b, err := io.ReadAll(p)
		require.NoError(t, err)
		dec, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(b), "\r\n", ""))
		require.NoError(t, err)
		types = append(types, p.Header.Get("Content-Type"))
		bodies = append(bodies, string(dec))
	}
	require.Len(t, bodies, 2)
	require.Contains(t, types[0], "text/plain")
	require.Contains(t, types[1], "text/html")
	require.Equal(t, "plain 123456", bodies[0])
	require.Equal(t, "<p>html 123456 🌸</p>", bodies[1])
}

func TestFormatFromKeepsDisplayName(t *testing.T) {
	h, addr := formatFrom("Bloom Health <no-reply@example.com>")
	require.Equal(t, "no-reply@example.com", addr)
	require.Contains(t, h, "Bloom Health")
}
