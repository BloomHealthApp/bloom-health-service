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

// LimitFriends applies the per-user limit on profile-link lookups and friend requests. It runs after Authenticate.
// LimitMessages limits sending messages to friends, per user (after auth).
func LimitMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		a := utils.GetActor(c.Request.Context())
		if a == nil || !di.AppLogic.GetRateLimiters().Messages.Allow(a.User.ID) {
			utils.RespondError(c, apperrors.TooManyRequestsError("too many messages, slow down", "5"))
			return
		}
		c.Next()
	}
}

func LimitFriends() gin.HandlerFunc {
	return func(c *gin.Context) {
		a := utils.GetActor(c.Request.Context())
		if a == nil || !di.AppLogic.GetRateLimiters().Friends.Allow(a.User.ID) {
			utils.RespondError(c, apperrors.TooManyRequestsError("too many requests, try again later", "30"))
			return
		}
		c.Next()
	}
}

// LimitProfilePage applies the per-IP limit on the public profile page.
func LimitProfilePage() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !di.AppLogic.GetRateLimiters().ProfilePage.Allow(ClientIP(c)) {
			utils.RespondError(c, apperrors.TooManyRequestsError("too many requests", "30"))
			return
		}
		c.Next()
	}
}
