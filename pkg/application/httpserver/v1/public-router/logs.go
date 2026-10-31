package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// ListLogs is GET /v1/logs?from&to (both required, max 366 days).
func ListLogs(c *gin.Context) {
	ctx := c.Request.Context()
	req := request.RangeReq{From: c.Query("from"), To: c.Query("to"), Required: true}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetDailyLogService().List(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PutLog is PUT /v1/logs/:habitId/:localDate.
func PutLog(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.PutLogReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	req.HabitID, req.LocalDate = c.Param("habitId"), c.Param("localDate")
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetDailyLogService().Put(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
