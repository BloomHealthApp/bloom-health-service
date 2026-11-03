package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleDeleteMe is DELETE /v1/me (both roles): permanently erases the caller's account and data.
func HandleDeleteMe(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetUserService().DeleteMe(ctx, utils.GetActor(ctx)); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
