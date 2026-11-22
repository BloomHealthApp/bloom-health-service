package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleGetProfileActivity is GET /v1/profiles/:id/activity: a friend's (or the caller's own) year of progress
// and today's habits.
func HandleGetProfileActivity(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetConnectionService().Activity(ctx, utils.GetActor(ctx), c.Param("id"))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
