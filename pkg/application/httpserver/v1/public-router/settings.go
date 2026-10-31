package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// GetSettings is GET /v1/settings (both roles).
func GetSettings(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetSettingsService().Get(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PatchSettings is PATCH /v1/settings (owner).
func PatchSettings(c *gin.Context) {
	ctx := c.Request.Context()
	req, ok := bindPatch(c, "")
	if !ok {
		return
	}
	resp, herr := di.AppLogic.GetSettingsService().Patch(ctx, utils.GetActor(ctx), req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
