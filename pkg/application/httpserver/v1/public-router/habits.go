package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// ListHabits is GET /v1/habits?includeInactive=bool.
func ListHabits(c *gin.Context) {
	ctx := c.Request.Context()
	req := request.ListHabitsReq{IncludeInactive: c.Query("includeInactive") == "true"}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetHabitService().List(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// bindCreate binds and validates a create body shared by habits and workouts.
func bindCreate(c *gin.Context) (*request.CreateEntityReq, bool) {
	var body map[string]any
	if herr := utils.BindStrictJSON(c, &body); herr != nil {
		utils.RespondError(c, herr)
		return nil, false
	}
	req := &request.CreateEntityReq{Payload: body}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return nil, false
	}
	return req, true
}

// bindPatch binds and validates a patch body; id is the path id (empty for settings).
func bindPatch(c *gin.Context, id string) (*request.PatchEntityReq, bool) {
	var body map[string]any
	if herr := utils.BindStrictJSON(c, &body); herr != nil {
		utils.RespondError(c, herr)
		return nil, false
	}
	req := &request.PatchEntityReq{ID: id, Payload: body}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return nil, false
	}
	return req, true
}

func CreateHabit(c *gin.Context) {
	ctx := c.Request.Context()
	req, ok := bindCreate(c)
	if !ok {
		return
	}
	resp, herr := di.AppLogic.GetHabitService().Create(ctx, utils.GetActor(ctx), req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// PatchHabit is PATCH /v1/habits/:id (optimistic version required).
func PatchHabit(c *gin.Context) {
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

// DeleteHabit is DELETE /v1/habits/:id (soft deactivate).
func DeleteHabit(c *gin.Context) {
	ctx := c.Request.Context()
	if herr := di.AppLogic.GetHabitService().Delete(ctx, utils.GetActor(ctx), c.Param("id")); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
