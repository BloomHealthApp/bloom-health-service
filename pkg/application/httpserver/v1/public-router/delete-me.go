package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleDeleteMe is DELETE /v1/me (both roles): locks the caller's account and schedules it for deletion.
func HandleDeleteMe(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetUserService().DeleteMe(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusAccepted, resp)
}
