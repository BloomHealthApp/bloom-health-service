package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bubu-bloom/service/pkg/di"
	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/dto/response"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/utils"
)

// OTPRequest is POST /v1/auth/otp/request. It answers 202 for every valid email so membership is never revealed.
func OTPRequest(c *gin.Context) {
	ctx := c.Request.Context()
	var req request.OTPRequestReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	sent := response.StatusResponse{Status: "sent"}
	if !di.AppLogic.GetRateLimiters().Email.Allow(req.Email) {
		c.JSON(http.StatusAccepted, sent) // same response as success: do not reveal anything
		return
	}
	// Failures are logged by the service; the caller always sees the same answer.
	_ = di.AppLogic.GetAuthService().RequestOTP(ctx, &req)
	c.JSON(http.StatusAccepted, sent)
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
	if !di.AppLogic.GetRateLimiters().Email.Allow("verify|" + req.Email) {
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
