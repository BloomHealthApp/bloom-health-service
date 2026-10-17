// Package config loads service configuration: non-secret settings from assets/<env>-config.json
// (selected by ACTIVE_ENV) with environment-variable overrides; secrets come only from the environment.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/bubu-bloom/service/assets"
	"github.com/bubu-bloom/service/pkg/constants"
)

// Store is the typed service configuration.
type Store struct {
	ActiveEnv  string // DEV | STAG | PROD
	Port       string
	TrustProxy bool
	// CORSOrigins lists allowed browser origins; empty (default) disables CORS headers.
	CORSOrigins []string
	Version     string
	LogLevel    string

	StoreKind string // mongo | memory
	MongoDB   string
	Issuer    string
	// OTPResendAfter is the cooldown before a new code may be issued to the same email.
	OTPResendAfter time.Duration

	Mailer   string // console | smtp
	SMTPPort string
	MailFrom string

	// DefaultTZ is the timezone assigned to new accounts until the app reports the device timezone.
	DefaultTZ string

	// Secrets and deployment-specific values: environment only, never in the JSON files.
	MongoURI     string
	AccessSecret string
	SMTPHost     string
	SMTPUser     string
	SMTPPass     string
}

// envBindings maps viper keys to the environment variables that may override them.
var envBindings = map[string]string{
	"server.port":             "PORT",
	"server.trustProxy":       "TRUST_PROXY",
	"log.level":               "LOG_LEVEL",
	"store.kind":              "STORE",
	"store.mongoDb":           "MONGODB_DB",
	"auth.issuer":             "AUTH_ISSUER",
	"mail.kind":               "MAILER",
	"mail.smtpPort":           "SMTP_PORT",
	"mail.from":               "MAIL_FROM",
	"account.defaultTimezone": "DEFAULT_TIMEZONE",
}

// NewConfig reads the file for ACTIVE_ENV (default DEV), applies env overrides and validates.
func NewConfig() (*Store, error) {
	env := strings.ToUpper(strings.TrimSpace(os.Getenv("ACTIVE_ENV")))
	if env == "" {
		env = constants.EnvDev
	}
	var file string
	switch env {
	case constants.EnvDev:
		file = "dev-config.json"
	case constants.EnvStag:
		file = "stag-config.json"
	case constants.EnvProd:
		file = "prod-config.json"
	default:
		return nil, fmt.Errorf("ACTIVE_ENV must be DEV, STAG or PROD, got %q", env)
	}
	raw, err := assets.Configs.ReadFile(file)
	if err != nil {
		return nil, err
	}
	v := viper.New()
	v.SetConfigType("json")
	if err := v.ReadConfig(bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	for key, name := range envBindings {
		if err := v.BindEnv(key, name); err != nil {
			return nil, err
		}
	}
	c := &Store{
		ActiveEnv: env, Port: v.GetString("server.port"), TrustProxy: v.GetBool("server.trustProxy"),
		CORSOrigins: splitList(v.GetString("server.corsOrigins")), Version: v.GetString("server.version"), LogLevel: v.GetString("log.level"),
		StoreKind: strings.ToLower(v.GetString("store.kind")), MongoDB: v.GetString("store.mongoDb"),
		Issuer: v.GetString("auth.issuer"), OTPResendAfter: time.Duration(v.GetInt("auth.otpResendSeconds")) * time.Second,
		Mailer: strings.ToLower(v.GetString("mail.kind")), SMTPPort: v.GetString("mail.smtpPort"), MailFrom: v.GetString("mail.from"),
		DefaultTZ: v.GetString("account.defaultTimezone"),
		MongoURI:  os.Getenv("MONGODB_URI"), AccessSecret: os.Getenv("ACCESS_TOKEN_SECRET"),
		SMTPHost: os.Getenv("SMTP_HOST"), SMTPUser: os.Getenv("SMTP_USER"), SMTPPass: os.Getenv("SMTP_PASS"),
	}
	return c, c.Validate()
}

// Validate checks required settings and production safety rules.
func (c *Store) Validate() error {
	if len(c.AccessSecret) < 32 {
		return errors.New("ACCESS_TOKEN_SECRET must be at least 32 characters")
	}
	if c.StoreKind != constants.StoreMongo && c.StoreKind != constants.StoreMemory {
		return fmt.Errorf("STORE must be mongo or memory, got %q", c.StoreKind)
	}
	if c.StoreKind == constants.StoreMongo && c.MongoURI == "" {
		return errors.New("MONGODB_URI is required when STORE=mongo")
	}
	if c.ActiveEnv == constants.EnvProd {
		if c.StoreKind != constants.StoreMongo {
			return errors.New("STORE=memory is not allowed in PROD")
		}
		if c.Mailer != "smtp" {
			return errors.New("MAILER=smtp is required in PROD (console mailer prints OTP codes)")
		}
	}
	if c.Mailer == "smtp" && c.SMTPHost == "" {
		return errors.New("SMTP_HOST is required when MAILER=smtp")
	}
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
