package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleDeclineConnection is POST /v1/connections/:id/decline (204).
func HandleDeclineConnection(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetConnectionService().Decline(ctx, utils.GetActor(ctx), c.Param("id")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
