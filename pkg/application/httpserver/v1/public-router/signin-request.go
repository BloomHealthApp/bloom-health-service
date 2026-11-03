package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleSignInRequest is POST /v1/auth/signin/request. It answers 202 with a challengeId for every
// well-formed identifier so account existence is never revealed.
func HandleSignInRequest(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.SignInReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Normalize(di.AppLogic.GetConfig().DefaultCountryCode); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	svc := di.AppLogic.GetAuthService()
	if !di.AppLogic.GetRateLimiters().Email.Allow("signin|" + req.Identifier) {
		// Over the per-identifier limit: same 202 shape, nothing is sent (the id is a decoy).
		req = request.SignInReq{Identifier: "decoy|" + uuid.NewString(), IsEmail: false}
	}
	resp, herr := svc.RequestSignIn(ctx, &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusAccepted, resp)
}
