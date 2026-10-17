// Package smtp delivers OTP emails. IMailer has an SMTP implementation, a console one for local
// development (it prints OTP codes, so production refuses to use it) and a capturing one for tests.
package smtp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"

	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/logger"
)

// IMailer sends a plain-text email.
type IMailer interface {
	// Send delivers a multipart message with a plain-text and an HTML body.
	Send(ctx context.Context, to, subject, text, html string) error
}

// NewMailer picks the implementation from configuration.
func NewMailer(cfg *config.Store, log logger.ILogger) IMailer {
	if cfg.Mailer == "smtp" {
		return &smtpMailer{host: cfg.SMTPHost, port: cfg.SMTPPort, user: cfg.SMTPUser, pass: cfg.SMTPPass, from: cfg.MailFrom}
	}
	return &consoleMailer{log: log}
}

type consoleMailer struct{ log logger.ILogger }

func (c *consoleMailer) Send(ctx context.Context, to, subject, text, _ string) error {
	c.log.Warn(ctx, "DEV MAILER (not sent)", map[string]any{"to": to, "subject": subject, "body": text})
	return nil
}

type smtpMailer struct{ host, port, user, pass, from string }

func (s *smtpMailer) Send(_ context.Context, to, subject, text, html string) error {
	for _, v := range []string{to, subject, s.from} {
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("invalid header value")
		}
	}
	fromHeader, fromAddr := formatFrom(s.from)
	msg, err := buildMessage(fromHeader, to, subject, text, html)
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}
	return smtp.SendMail(s.host+":"+s.port, auth, fromAddr, []string{to}, msg)
}

// formatFrom returns the From header (display name "Bubu Bloom" added when missing) and the bare address.
func formatFrom(from string) (header, addr string) {
	a, err := mail.ParseAddress(from)
	if err != nil {
		return from, from
	}
	if a.Name == "" {
		a.Name = "Bubu Bloom"
	}
	return a.String(), a.Address
}

// buildMessage renders a multipart/alternative message (text first, HTML preferred) with base64 bodies.
func buildMessage(from, to, subject, text, html string) ([]byte, error) {
	boundary := "bubu-" + hex.EncodeToString(randBytes(12))
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
	for _, part := range []struct{ ctype, body string }{{"text/plain", text}, {"text/html", html}} {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + part.ctype + "; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
		enc := base64.StdEncoding.EncodeToString([]byte(part.body))
		for len(enc) > 76 {
			b.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		b.WriteString(enc + "\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String()), nil
}

func randBytes(n int) []byte {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return buf
}

type Message struct{ To, Subject, Body, HTML string }

// CaptureMailer records messages in memory (tests).
type CaptureMailer struct {
	mu       sync.Mutex
	Messages []Message
}

// Send records the message.
func (c *CaptureMailer) Send(_ context.Context, to, subject, text, html string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Messages = append(c.Messages, Message{to, subject, text, html})
	return nil
}

// Reset drops captured messages.
func (c *CaptureMailer) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Messages = nil
}

// Snapshot returns a copy of the captured messages.
func (c *CaptureMailer) Snapshot() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Message(nil), c.Messages...)
}
