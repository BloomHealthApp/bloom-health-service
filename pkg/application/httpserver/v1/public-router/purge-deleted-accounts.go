package publicrouter

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandlePurgeDeletedAccounts is GET /internal/cron/purge-deleted-accounts, called daily by Vercel Cron with
// "Authorization: Bearer $CRON_SECRET". It erases every account whose deletion grace period has ended.
func HandlePurgeDeletedAccounts(c *gin.Context) {
	secret := di.AppLogic.GetConfig().CronSecret
	if secret == "" {
		utils.RespondError(c, apperrors.NotFoundError("no such route"))
		return
	}
	// Security: constant-time compare so the secret can't be guessed byte by byte from response timing.
	if subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+secret)) != 1 {
		utils.RespondError(c, apperrors.UnauthorizedError("missing or invalid cron secret"))
		return
	}
	n, herr := di.AppLogic.GetUserService().PurgeDue(c.Request.Context())
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"erased": n})
}
