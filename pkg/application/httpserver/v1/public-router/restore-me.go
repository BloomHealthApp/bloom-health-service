package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleRestoreMe is POST /v1/me/restore (both roles): cancels a scheduled deletion during the grace period.
func HandleRestoreMe(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetUserService().RestoreMe(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
