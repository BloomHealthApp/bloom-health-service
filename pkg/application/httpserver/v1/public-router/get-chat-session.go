package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleGetChatSession is GET /v1/chat/sessions/:sid.
func HandleGetChatSession(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetChatService().GetSession(ctx, utils.GetActor(ctx), c.Param("sid"))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
