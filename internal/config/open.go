package config

import (
	"context"
	"log/slog"

	"github.com/bubu-bloom/service/internal/mailer"
	"github.com/bubu-bloom/service/internal/store"
)

// OpenStore connects the configured persistence backend.
func (c *Config) OpenStore(ctx context.Context) (store.Store, error) {
	if c.StoreKind == "memory" {
		return store.NewMemory(), nil
	}
	return store.NewMongo(ctx, c.MongoURI, c.MongoDB)
}

func (c *Config) NewMailer(log *slog.Logger) mailer.Mailer {
	if c.Mailer == "smtp" {
		return mailer.SMTP{Host: c.SMTPHost, Port: c.SMTPPort, User: c.SMTPUser, Pass: c.SMTPPass, From: c.MailFrom}
	}
	return mailer.Console{Log: log}
}
