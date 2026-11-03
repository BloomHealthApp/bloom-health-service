package publicrouter

import (
	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

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
