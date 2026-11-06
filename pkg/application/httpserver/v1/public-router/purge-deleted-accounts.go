package publicrouter

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// HandlePurgeDeletedAccounts is POST /internal/accounts/purge-deleted, called daily at 10:00 IST by
// bloom-health-task-worker with "Authorization: Bearer $INTERNAL_API_SECRET". It erases every account whose
// deletion grace period has ended and answers {"erased": n}.
func HandlePurgeDeletedAccounts(c *gin.Context) {
	secret := di.AppLogic.GetConfig().InternalAPISecret
	if secret == "" {
		utils.RespondError(c, apperrors.NotFoundError("no such route"))
		return
	}
	// Security: constant-time compare so the secret can't be guessed byte by byte from response timing.
	if subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+secret)) != 1 {
		utils.RespondError(c, apperrors.UnauthorizedError("missing or invalid internal API secret"))
		return
	}
	n, herr := di.AppLogic.GetUserService().PurgeDue(c.Request.Context())
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"erased": n})
}
