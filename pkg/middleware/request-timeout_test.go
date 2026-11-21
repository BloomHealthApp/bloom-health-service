package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestTimeoutSetsDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var left time.Duration
	var hasDeadline bool
	r.GET("/x", RequestTimeout(2*time.Second), func(c *gin.Context) {
		var dl time.Time
		dl, hasDeadline = c.Request.Context().Deadline()
		left = time.Until(dl)
		c.Status(http.StatusNoContent)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	require.True(t, hasDeadline)
	require.InDelta(t, (2 * time.Second).Seconds(), left.Seconds(), 0.5)
}
