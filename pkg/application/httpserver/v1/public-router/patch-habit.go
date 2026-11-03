package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandlePatchHabit is PATCH /v1/habits/:id (optimistic version required).
func HandlePatchHabit(c *gin.Context) {
	ctx := c.Request.Context()
	req, ok := bindPatch(c, c.Param("id"))
	if !ok {
		return
	}
	resp, herr := di.AppLogic.GetHabitService().Patch(ctx, utils.GetActor(ctx), req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
