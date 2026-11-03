package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleSignUpRequest is POST /v1/auth/signup/request: 202 {status, challengeId}, 409 already_registered.
func HandleSignUpRequest(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.SignUpReq
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
	lim := di.AppLogic.GetRateLimiters().Email
	if !lim.Allow("signup|"+req.Email) || !lim.Allow("signup|"+req.Phone) {
		utils.RespondError(c, apperrors.TooManyRequestsError("too many attempts, try again later", "60"))
		return
	}
	resp, herr := di.AppLogic.GetAuthService().RequestSignUp(ctx, &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusAccepted, resp)
}
