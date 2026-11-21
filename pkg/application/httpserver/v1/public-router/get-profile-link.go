package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleGetProfileLink is GET /v1/me/profile-link: the caller's shareable profile link.
func HandleGetProfileLink(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetConnectionService().ProfileLink(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
