package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
)

// CORS answers browser preflight requests for the configured origins. With no origins configured
// (the default; the mobile app is not a browser) it adds no headers at all.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origins := di.AppLogic.GetConfig().CORSOrigins
		origin := c.GetHeader("Origin")
		if len(origins) == 0 || origin == "" {
			c.Next()
			return
		}
		allowed := false
		for _, o := range origins {
			if o == "*" || strings.EqualFold(o, origin) {
				allowed = true
			}
		}
		if !allowed {
			c.Next()
			return
		}
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Add("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id,X-Device-Id,X-Client-Type,X-App-Version,X-Timezone")
			h.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
