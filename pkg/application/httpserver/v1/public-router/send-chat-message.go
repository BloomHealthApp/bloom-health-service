package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleSendChatMessage is POST /v1/chat/sessions/:sid/messages: 202 with the stored user message and the
// pending assistant message, which bloom-health-task-worker answers.
func HandleSendChatMessage(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.SendChatMessageReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	req.SessionID = c.Param("sid")
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetChatService().SendMessage(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusAccepted, resp)
}
