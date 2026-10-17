package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bubu-bloom/service/pkg/di"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/utils"
)

// Authenticate verifies the bearer token and resolves user and membership from the database on every
// request; the role is never taken from the client or the token. The actor is stored in the context.
func Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			utils.RespondError(c, apperrors.UnauthorizedError("missing bearer token"))
			return
		}
		ctx := c.Request.Context()
		actor, herr := di.AppLogic.GetAuthService().Authenticate(ctx, strings.TrimPrefix(h, "Bearer "))
		if herr != nil {
			utils.RespondError(c, herr)
			return
		}
		c.Request = c.Request.WithContext(utils.SetInContext(ctx, utils.ActorKey, actor))
		c.Next()
	}
}

// RequireOwner rejects viewers with 403 before the handler runs (services enforce it again).
func RequireOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		a := utils.GetActor(c.Request.Context())
		if a == nil || !a.IsOwner() {
			utils.RespondError(c, apperrors.ForbiddenError("read-only access"))
			return
		}
		c.Next()
	}
}
