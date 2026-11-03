package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleDeleteWorkout is DELETE /v1/workouts/:id (tombstone).
func HandleDeleteWorkout(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetWorkoutService().Delete(ctx, utils.GetActor(ctx), c.Param("id")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
