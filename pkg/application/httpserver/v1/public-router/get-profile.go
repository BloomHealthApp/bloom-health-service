package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleGetProfile is GET /v1/profiles/:id: a friend's (or the caller's own) public profile.
func HandleGetProfile(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetConnectionService().Profile(ctx, utils.GetActor(ctx), c.Param("id"))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
