package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

func GetMe(c *gin.Context) {
	ctx := c.Request.Context()
	resp, herr := di.AppLogic.GetUserService().GetMe(ctx, utils.GetActor(ctx))
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PatchMe is PATCH /v1/me (both roles; only the caller's own profile).
func PatchMe(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.PatchMeReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetUserService().PatchMe(ctx, utils.GetActor(ctx), &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// RequestPhoneChange is POST /v1/me/phone/request: mails a code to the account's email (202 {status, challengeId}).
func RequestPhoneChange(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.PhoneChangeReq
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
	actor := utils.GetActor(ctx)
	if !di.AppLogic.GetRateLimiters().Email.Allow("phone|" + actor.User.ID) {
		utils.RespondError(c, apperrors.TooManyRequestsError("too many attempts, try again later", "60"))
		return
	}
	resp, herr := di.AppLogic.GetAuthService().RequestPhoneChange(ctx, actor, &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusAccepted, resp)
}

// VerifyPhoneChange is POST /v1/me/phone/verify: checks the code, then saves the phone and returns the user.
func VerifyPhoneChange(c *gin.Context) {
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
