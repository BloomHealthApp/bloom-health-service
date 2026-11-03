package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleListChatMessages is GET /v1/chat/sessions/:sid/messages?before&limit.
func HandleListChatMessages(c *gin.Context) {
	ctx := c.Request.Context()
	req := request.ListChatMessagesReq{SessionID: c.Param("sid"), RawBefore: c.Query("before"), RawLimit: c.Query("limit")}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetChatService().ListMessages(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
