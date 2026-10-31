package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// Recovery turns panics into the contract's 500 internal error without leaking details.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				di.AppLogic.GetLogger().Error(c.Request.Context(), "panic", map[string]any{"value": rec, "route": routeOf(c)})
				utils.RespondError(c, apperrors.InternalServerError("something went wrong"))
			}
		}()
		c.Next()
	}
}

func routeOf(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return "unmatched"
}
