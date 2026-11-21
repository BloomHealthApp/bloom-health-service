package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestTimeout gives each request a deadline, which database calls honour through the request context.
func RequestTimeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
