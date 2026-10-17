package logger

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bubu-bloom/service/pkg/utils"
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
