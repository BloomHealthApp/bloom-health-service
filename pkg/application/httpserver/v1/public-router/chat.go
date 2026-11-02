package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// ListChatModels is GET /v1/chat/models.
func ListChatModels(c *gin.Context) {
	c.JSON(http.StatusOK, di.AppLogic.GetChatService().ListModels(c.Request.Context()))
}

// CreateChatSession is POST /v1/chat/sessions.
func CreateChatSession(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.CreateChatSessionReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetChatService().CreateSession(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// ListChatSessions is GET /v1/chat/sessions (the caller's own, most recent first).
func ListChatSessions(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetChatService().ListSessions(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetChatSession is GET /v1/chat/sessions/:sid.
func GetChatSession(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetChatService().GetSession(ctx, utils.GetActor(ctx), c.Param("sid"))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteChatSession is DELETE /v1/chat/sessions/:sid (soft delete).
func DeleteChatSession(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetChatService().DeleteSession(ctx, utils.GetActor(ctx), c.Param("sid")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListChatMessages is GET /v1/chat/sessions/:sid/messages?before&limit.
func ListChatMessages(c *gin.Context) {
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

// SendChatMessage is POST /v1/chat/sessions/:sid/messages: 202 with the stored user message and the
// pending assistant message, which bloom-health-task-worker answers.
func SendChatMessage(c *gin.Context) {
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
