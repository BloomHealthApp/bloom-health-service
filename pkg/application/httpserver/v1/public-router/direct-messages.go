package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleListDMThreads is GET /v1/dm/threads: the caller's chats with friends.
func HandleListDMThreads(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetDMService().Threads(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// HandleListDMs is GET /v1/dm/:id/messages?before&after&limit.
func HandleListDMs(c *gin.Context) {
	ctx := c.Request.Context()
	req := request.ListDMsReq{FriendID: c.Param("id"), RawBefore: c.Query("before"), RawAfter: c.Query("after"), RawLimit: c.Query("limit")}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetDMService().Messages(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// HandleSendDM is POST /v1/dm/:id/messages: 201 for a new message, 200 for a resend of one already saved.
func HandleSendDM(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.SendDMReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	req.FriendID = c.Param("id")
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, created, herr := di.AppLogic.GetDMService().Send(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, resp)
}

// HandleMarkDMRead is POST /v1/dm/:id/read.
func HandleMarkDMRead(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetDMService().MarkRead(ctx, utils.GetActor(ctx), c.Param("id")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleSavePushToken is PUT /v1/me/push-token.
func HandleSavePushToken(c *gin.Context) {
	handlePushToken(c, true)
}

// HandleDeletePushToken is DELETE /v1/me/push-token.
func HandleDeletePushToken(c *gin.Context) {
	handlePushToken(c, false)
}

func handlePushToken(c *gin.Context, saving bool) {
	ctx := c.Request.Context()
	var req request.PushTokenReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(saving); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	svc := di.AppLogic.GetDMService()
	save := svc.DeletePushToken
	if saving {
		save = svc.SavePushToken
	}
	if herr := save(ctx, utils.GetActor(ctx), &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
