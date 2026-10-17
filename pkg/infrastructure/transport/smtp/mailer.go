// Package smtp delivers OTP emails. IMailer has an SMTP implementation, a console one for local
// development (it prints OTP codes, so production refuses to use it) and a capturing one for tests.
package smtp

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"sync"

	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/logger"
)

// IMailer sends a plain-text email.
type IMailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// NewMailer picks the implementation from configuration.
func NewMailer(cfg *config.Store, log logger.ILogger) IMailer {
	if cfg.Mailer == "smtp" {
		return &smtpMailer{host: cfg.SMTPHost, port: cfg.SMTPPort, user: cfg.SMTPUser, pass: cfg.SMTPPass, from: cfg.MailFrom}
	}
	return &consoleMailer{log: log}
}

type consoleMailer struct{ log logger.ILogger }

func (c *consoleMailer) Send(ctx context.Context, to, subject, body string) error {
	c.log.Warn(ctx, "DEV MAILER (not sent)", map[string]any{"to": to, "subject": subject, "body": body})
	return nil
}

type smtpMailer struct{ host, port, user, pass, from string }

func (s *smtpMailer) Send(_ context.Context, to, subject, body string) error {
	for _, v := range []string{to, subject, s.from} {
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("invalid header value")
		}
	}
	msg := "From: " + s.from + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body
	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}
	from := s.from
	if i := strings.Index(from, "<"); i >= 0 {
		from = strings.TrimSuffix(from[i+1:], ">")
	}
	return smtp.SendMail(s.host+":"+s.port, auth, from, []string{to}, []byte(msg))
}

// Message is a captured email.
type Message struct{ To, Subject, Body string }

// CaptureMailer records messages in memory (tests).
type CaptureMailer struct {
	mu       sync.Mutex
	Messages []Message
}

// Send records the message.
func (c *CaptureMailer) Send(_ context.Context, to, subject, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Messages = append(c.Messages, Message{to, subject, body})
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
