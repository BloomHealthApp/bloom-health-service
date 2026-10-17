// Package logger provides a context-first structured logger over log/slog. Fields whose names look
// sensitive (tokens, codes, secrets, hashes) are redacted so wellness data and credentials never reach logs.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/utils"
)

// ILogger is the logging contract used by every layer. The context supplies the request id.
type ILogger interface {
	Debug(ctx context.Context, msg string, fields map[string]any)
	Info(ctx context.Context, msg string, fields map[string]any)
	Warn(ctx context.Context, msg string, fields map[string]any)
	Error(ctx context.Context, msg string, fields map[string]any)
}

type slogLogger struct{ l *slog.Logger }

var sensitiveParts = []string{"token", "otp", "code", "secret", "password", "pass", "authorization", "hash", "uri",
	// private health/profile data (period tracker, gender) must never reach logs
	"period", "gender", "symptom", "mood", "flow", "startdate", "enddate"}

const redacted = "[REDACTED]"

// NewLogger builds the JSON logger on stdout at the configured level.
func NewLogger(cfg *config.Store) ILogger { return NewWithWriter(os.Stdout, cfg.LogLevel) }

// NewWithWriter builds a logger writing JSON lines to w (level: debug|info|warn|error).
func NewWithWriter(w io.Writer, level string) ILogger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return &slogLogger{l: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv}))}
}

// NewDiscard returns a logger that drops everything (tests).
func NewDiscard() ILogger { return NewWithWriter(io.Discard, "error") }

func isSensitive(key string) bool {
	k := strings.ToLower(key)
	for _, p := range sensitiveParts {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

func (s *slogLogger) log(ctx context.Context, lv slog.Level, msg string, fields map[string]any) {
	if !s.l.Enabled(ctx, lv) {
		return
	}
	attrs := make([]any, 0, len(fields)*2+2)
	if id := utils.GetFromContext(ctx, utils.RequestIDKey); id != "" {
		attrs = append(attrs, "requestId", id)
	}
	for k, v := range fields {
		if isSensitive(k) {
			v = redacted
		}
		attrs = append(attrs, k, v)
	}
	s.l.Log(ctx, lv, msg, attrs...)
}

func (s *slogLogger) Debug(ctx context.Context, msg string, f map[string]any) {
	s.log(ctx, slog.LevelDebug, msg, f)
}
func (s *slogLogger) Info(ctx context.Context, msg string, f map[string]any) {
	s.log(ctx, slog.LevelInfo, msg, f)
}
func (s *slogLogger) Warn(ctx context.Context, msg string, f map[string]any) {
	s.log(ctx, slog.LevelWarn, msg, f)
}
func (s *slogLogger) Error(ctx context.Context, msg string, f map[string]any) {
	s.log(ctx, slog.LevelError, msg, f)
}
