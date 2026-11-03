package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleVerifyOTP is POST /v1/auth/otp/verify: checks the code and returns tokens and the user.
func HandleVerifyOTP(c *gin.Context) {
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
	if !di.AppLogic.GetRateLimiters().Email.Allow("verify|" + req.ChallengeID) {
		utils.RespondError(c, apperrors.TooManyRequestsError("too many attempts, try again later", "60"))
		return
	}
	resp, herr := di.AppLogic.GetAuthService().VerifyOTP(ctx, &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
