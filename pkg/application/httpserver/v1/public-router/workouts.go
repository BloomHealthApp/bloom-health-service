package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// ListWorkouts is GET /v1/workouts?from&to.
func ListWorkouts(c *gin.Context) {
	ctx := c.Request.Context()
	req := request.RangeReq{From: c.Query("from"), To: c.Query("to")}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetWorkoutService().List(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CreateWorkout is POST /v1/workouts.
func CreateWorkout(c *gin.Context) {
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

// PatchWorkout is PATCH /v1/workouts/:id (optimistic version required).
func PatchWorkout(c *gin.Context) {
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

// DeleteWorkout is DELETE /v1/workouts/:id (tombstone).
func DeleteWorkout(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetWorkoutService().Delete(ctx, utils.GetActor(ctx), c.Param("id")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
