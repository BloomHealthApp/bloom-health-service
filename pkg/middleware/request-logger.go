package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
)

// RequestLogger logs one line per request. It records the route pattern, never paths, queries or bodies.
// Successful Kubernetes probes are not logged: they arrive every few seconds per pod and would drown real traffic.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if isProbe(c) {
			return
		}
		di.AppLogic.GetLogger().Info(c.Request.Context(), "request", map[string]any{
			"method": c.Request.Method, "route": routeOf(c), "status": c.Writer.Status(),
			"ms": time.Since(start).Milliseconds(),
		})
	}
}

func isProbe(c *gin.Context) bool {
	route := routeOf(c)
	return (route == "/livez" || route == "/health") && c.Writer.Status() < 400 &&
		strings.HasPrefix(c.Request.UserAgent(), "kube-probe/")
}
