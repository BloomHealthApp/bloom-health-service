package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
)

// HandleListChatModels is GET /v1/chat/models.
func HandleListChatModels(c *gin.Context) {
	c.JSON(http.StatusOK, di.AppLogic.GetChatService().ListModels(c.Request.Context()))
}
