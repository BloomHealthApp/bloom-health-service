package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bubu-bloom/service/pkg/di"
	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/utils"
)

// SyncPush is POST /v1/sync/push (owner only).
func SyncPush(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.SyncPushReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetSyncService().Push(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// SyncPull is GET /v1/sync/pull?cursor&limit (both roles).
func SyncPull(c *gin.Context) {
	ctx := c.Request.Context()
	req := request.SyncPullReq{Cursor: c.Query("cursor"), RawLimit: c.Query("limit")}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetSyncService().Pull(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
