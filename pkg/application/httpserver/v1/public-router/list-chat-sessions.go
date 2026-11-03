package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleListChatSessions is GET /v1/chat/sessions (the caller's own, most recent first).
func HandleListChatSessions(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetChatService().ListSessions(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
