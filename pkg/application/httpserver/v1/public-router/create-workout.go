package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleCreateWorkout is POST /v1/workouts (owner).
func HandleCreateWorkout(c *gin.Context) {
	ctx := c.Request.Context()
	req, ok := bindCreate(c)
	if !ok {
		return
	}
	resp, herr := di.AppLogic.GetWorkoutService().Create(ctx, utils.GetActor(ctx), req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusCreated, resp)
}
