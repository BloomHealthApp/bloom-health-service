package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bubu-bloom/service/pkg/di"
	"github.com/bubu-bloom/service/pkg/utils"
)

// Health is GET /health and GET /v1/health (no auth, no database detail).
func Health(c *gin.Context) {
	resp, herr := di.AppLogic.GetHealthService().Check(c.Request.Context())
	if herr != nil {
		utils.RespondError(c, herr)
		return
	}
	c.JSON(http.StatusOK, resp)
}
