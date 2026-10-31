package middleware

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// ClientIP returns the caller's address: the first X-Forwarded-For entry only when TRUST_PROXY is
// enabled (a trusted proxy sets it), otherwise the TCP peer.
func ClientIP(c *gin.Context) string {
	if di.AppLogic.GetConfig().TrustProxy {
		if xf := c.GetHeader("X-Forwarded-For"); xf != "" {
			return strings.TrimSpace(strings.Split(xf, ",")[0])
		}
	}
	h, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return h
}

// LimitIP applies the global per-IP limit.
func LimitIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !di.AppLogic.GetRateLimiters().IP.Allow(ClientIP(c)) {
			utils.RespondError(c, apperrors.TooManyRequestsError("too many requests", "5"))
			return
		}
		c.Next()
	}
}

// LimitAuth applies the stricter per-IP limit to /auth/*. Per-email limits live in the handlers
// because they depend on the request body.
func LimitAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !di.AppLogic.GetRateLimiters().Auth.Allow(ClientIP(c)) {
			utils.RespondError(c, apperrors.TooManyRequestsError("too many attempts, try again later", "30"))
			return
		}
		c.Next()
	}
}
