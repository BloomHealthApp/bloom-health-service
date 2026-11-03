package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleDeleteChatSession is DELETE /v1/chat/sessions/:sid (soft delete).
func HandleDeleteChatSession(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetChatService().DeleteSession(ctx, utils.GetActor(ctx), c.Param("sid")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
