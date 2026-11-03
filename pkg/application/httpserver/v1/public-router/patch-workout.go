package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandlePatchWorkout is PATCH /v1/workouts/:id (optimistic version required).
func HandlePatchWorkout(c *gin.Context) {
	ctx := c.Request.Context()
	req, ok := bindPatch(c, c.Param("id"))
	if !ok {
		return
	}
	resp, herr := di.AppLogic.GetWorkoutService().Patch(ctx, utils.GetActor(ctx), req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
