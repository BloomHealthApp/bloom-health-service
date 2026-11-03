package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandlePatchMe is PATCH /v1/me (both roles; only the caller's own profile).
func HandlePatchMe(c *gin.Context) {
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
	resp, herr := di.AppLogic.GetUserService().PatchMe(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
