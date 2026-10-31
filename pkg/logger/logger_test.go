package logger

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

func TestSensitiveFieldsAreRedacted(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf, "info")
	ctx := utils.SetInContext(context.Background(), utils.RequestIDKey, "req-1")
	l.Info(ctx, "msg", map[string]any{"accessToken": "secret-token", "otp": "123456", "refreshToken": "r", "route": "/v1/me"})
	out := buf.String()
	require.NotContains(t, out, "secret-token")
	require.NotContains(t, out, "123456")
	require.Contains(t, out, `"route":"/v1/me"`)
	require.Contains(t, out, `"requestId":"req-1"`)
}

func TestPeriodAndGenderFieldsAreRedacted(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf, "info")
	l.Info(context.Background(), "msg", map[string]any{"periodNote": "private-a", "gender": "female", "symptoms": "private-b",
		"startDate": "2026-10-01", "endDate": "2026-10-05", "mood": "sad", "flow": "heavy", "route": "/v1/periods"})
	out := buf.String()
	for _, s := range []string{"private-a", "female", "private-b", "2026-10-01", "2026-10-05", "sad", "heavy"} {
		require.NotContains(t, out, s)
	}
	require.Contains(t, out, `"route":"/v1/periods"`)
}
