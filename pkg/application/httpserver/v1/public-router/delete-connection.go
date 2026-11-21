package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleDeleteConnection is DELETE /v1/connections/:id: unfriend, or withdraw the caller's own request (204).
func HandleDeleteConnection(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetConnectionService().Remove(ctx, utils.GetActor(ctx), c.Param("id")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
