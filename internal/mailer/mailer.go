// Package mailer delivers OTP emails.
package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Console logs messages. For local development only (it prints OTP codes).
type Console struct{ Log *slog.Logger }

func (c Console) Send(_ context.Context, to, subject, body string) error {
	c.Log.Warn("DEV MAILER (not sent)", "to", to, "subject", subject, "body", body)
	return nil
}

type SMTP struct{ Host, Port, User, Pass, From string }

func (s SMTP) Send(_ context.Context, to, subject, body string) error {
	for _, v := range []string{to, subject, s.From} {
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("invalid header value")
		}
	}
	msg := "From: " + s.From + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body
	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Pass, s.Host)
	}
	from := s.From
	if i := strings.Index(from, "<"); i >= 0 {
		from = strings.TrimSuffix(from[i+1:], ">")
	}
	return smtp.SendMail(s.Host+":"+s.Port, auth, from, []string{to}, []byte(msg))
}

// Capture records messages in memory (tests).
type Capture struct{ Messages []Message }
type Message struct{ To, Subject, Body string }

func (c *Capture) Send(_ context.Context, to, subject, body string) error {
	c.Messages = append(c.Messages, Message{to, subject, body})
	return nil
}
