package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bubu-bloom/service/pkg/di"
	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/utils"
)

// GetMe is GET /v1/me.
func GetMe(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetUserService().GetMe(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PatchMe is PATCH /v1/me (both roles; only the caller's own profile).
func PatchMe(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.PatchMeReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Normalize(di.AppLogic.GetConfig().DefaultCountryCode); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetUserService().PatchMe(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
