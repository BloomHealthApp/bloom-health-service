package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleRequestPhoneChange is POST /v1/me/phone/request: mails a code to the account's email (202 {status, challengeId}).
func HandleRequestPhoneChange(c *gin.Context) {
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
