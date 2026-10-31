package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
)

// RequestLogger logs one line per request. It records the route pattern, never paths, queries or bodies.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		di.AppLogic.GetLogger().Info(c.Request.Context(), "request", map[string]any{
			"method": c.Request.Method, "route": routeOf(c), "status": c.Writer.Status(),
			"ms": time.Since(start).Milliseconds(),
		})
	}
}
