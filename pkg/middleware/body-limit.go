package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
)

// BodyLimit caps request bodies at 1 MiB; reads beyond it fail and are answered with 400.
func BodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, constants.MaxBodyBytes)
		c.Next()
	}
}
