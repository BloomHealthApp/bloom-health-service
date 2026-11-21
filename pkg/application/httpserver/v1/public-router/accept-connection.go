package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleAcceptConnection is POST /v1/connections/:id/accept.
func HandleAcceptConnection(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetConnectionService().Accept(ctx, utils.GetActor(ctx), c.Param("id"))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
