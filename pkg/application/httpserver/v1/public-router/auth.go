package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/bubu-bloom/service/pkg/di"
	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/utils"
)

// SignUpRequest is POST /v1/auth/signup/request: 202 {status, challengeId}, 409 already_registered.
func SignUpRequest(c *gin.Context) {
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

// SignInRequest is POST /v1/auth/signin/request. It answers 202 with a challengeId for every
// well-formed identifier so account existence is never revealed.
func SignInRequest(c *gin.Context) {
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

// OTPVerify is POST /v1/auth/otp/verify.
func OTPVerify(c *gin.Context) {
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

// Refresh is POST /v1/auth/refresh.
func Refresh(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.RefreshTokenReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetAuthService().Refresh(ctx, &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Logout is POST /v1/auth/logout.
func Logout(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.RefreshTokenReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := di.AppLogic.GetAuthService().Logout(ctx, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.Status(http.StatusNoContent)
}
