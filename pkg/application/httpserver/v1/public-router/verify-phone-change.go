package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleVerifyPhoneChange is POST /v1/me/phone/verify: checks the code, then saves the phone and returns the user.
func HandleVerifyPhoneChange(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.OTPVerifyReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	actor := utils.GetActor(ctx)
	if !di.AppLogic.GetRateLimiters().Email.Allow("verify|" + req.ChallengeID) {
		utils.RespondError(c, apperrors.TooManyRequestsError("too many attempts, try again later", "60"))
		return
	}
	phone, herr := di.AppLogic.GetAuthService().ConfirmPhoneChange(ctx, actor, &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetUserService().SetPhone(ctx, actor, phone)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
