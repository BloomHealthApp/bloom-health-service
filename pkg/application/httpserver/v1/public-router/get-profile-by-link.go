package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleGetProfileByLink is GET /v1/profiles/by-link/:code: the profile behind a link and the caller's relation to it.
func HandleGetProfileByLink(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetConnectionService().ProfileByLink(ctx, utils.GetActor(ctx), c.Param("code"))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
