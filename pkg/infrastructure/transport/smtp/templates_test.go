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

func TestOTPEmailContent(t *testing.T) {
	subject, text, html := OTPEmail("123456", 10)
	require.Equal(t, "Your Bubu Bloom sign-in code", subject)
	require.Contains(t, text, "123456")
	require.Contains(t, text, "10 minutes")
	require.Contains(t, html, "<!DOCTYPE html>")
	require.Contains(t, html, "Bubu Bloom")
	require.Contains(t, html, "10 minutes")
	require.Contains(t, html, "&#8202;", "digits are spaced for readability")
	require.NotContains(t, html, "<script")
	require.NotContains(t, html, "http://", "no remote resources")
	require.NotContains(t, html, "%!", "no formatting errors")
}

func TestOTPEmailEscapesCode(t *testing.T) {
	_, _, html := OTPEmail(`<b>"&`, 5)
	require.NotContains(t, html, "<b>")
	require.Contains(t, html, "&lt;b&gt;")
}

func TestBuildMessageIsValidMultipart(t *testing.T) {
	from, _ := formatFrom("apcode15@gmail.com")
	require.Equal(t, `"Bubu Bloom" <apcode15@gmail.com>`, from)
	raw, err := buildMessage(from, "user@example.com", "Your Bubu Bloom sign-in code", "plain 123456", "<p>html 123456 🌸</p>")
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
	h, addr := formatFrom("Bubu Bloom <apcode15@gmail.com>")
	require.Equal(t, "apcode15@gmail.com", addr)
	require.Contains(t, h, "Bubu Bloom")
}
