// Package config loads service configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env          string // development | production
	Port         string
	StoreKind    string // mongo | memory
	MongoURI     string
	MongoDB      string
	AccessSecret string
	Issuer       string
	Mailer       string // console | smtp
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPass     string
	MailFrom     string
	TrustProxy   bool
	// Seed provisioning
	OwnerEmail, OwnerName, ViewerEmail, ViewerName, DefaultTZ string
}

func get(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func Load() (*Config, error) {
	c := &Config{
		Env: get("APP_ENV", "development"), Port: get("PORT", "3000"),
		StoreKind: get("STORE", "mongo"), MongoURI: os.Getenv("MONGODB_URI"), MongoDB: get("MONGODB_DB", "bubu_bloom"),
		AccessSecret: os.Getenv("ACCESS_TOKEN_SECRET"), Issuer: get("AUTH_ISSUER", "bubu-bloom"),
		Mailer: get("MAILER", "console"), SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: get("SMTP_PORT", "587"),
		SMTPUser: os.Getenv("SMTP_USER"), SMTPPass: os.Getenv("SMTP_PASS"), MailFrom: get("MAIL_FROM", "Bubu Bloom <no-reply@localhost>"),
		OwnerEmail: os.Getenv("OWNER_EMAIL"), OwnerName: get("OWNER_NAME", "Anu"),
		ViewerEmail: os.Getenv("VIEWER_EMAIL"), ViewerName: get("VIEWER_NAME", "Aman"),
		DefaultTZ: get("DEFAULT_TIMEZONE", "Asia/Kolkata"),
	}
	c.TrustProxy, _ = strconv.ParseBool(os.Getenv("TRUST_PROXY"))
	return c, c.Validate()
}

func (c *Config) Validate() error {
	if len(c.AccessSecret) < 32 {
		return errors.New("ACCESS_TOKEN_SECRET must be at least 32 characters")
	}
	if c.StoreKind == "mongo" && c.MongoURI == "" {
		return errors.New("MONGODB_URI is required when STORE=mongo")
	}
	if c.Env == "production" {
		if c.StoreKind != "mongo" {
			return errors.New("STORE=memory is not allowed in production")
		}
		if c.Mailer != "smtp" {
			return errors.New("MAILER=smtp is required in production (console mailer prints OTP codes)")
		}
	}
	if c.Mailer == "smtp" && c.SMTPHost == "" {
		return fmt.Errorf("SMTP_HOST is required when MAILER=smtp")
	}
	return nil
}
