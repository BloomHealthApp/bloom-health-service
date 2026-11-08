package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandleGoogleSignIn is POST /v1/auth/google: signs in (linking or creating the account) with a Google ID token
// and returns tokens and the user. A 404 when GOOGLE_CLIENT_ID is unset.
func HandleGoogleSignIn(c *gin.Context) {
	if len(di.AppLogic.GetConfig().GoogleClientIDs) == 0 {
		utils.RespondError(c, apperrors.NotFoundError("no such route"))
		return
	}
	ctx := c.Request.Context()
	var req request.GoogleSignInReq
	if herr := utils.BindStrictJSON(c, &req); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	if herr := req.Validate(); herr != nil {
		utils.RespondError(c, herr)
		return
	}
	resp, herr := di.AppLogic.GetAuthService().SignInWithGoogle(ctx, &req)
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
