package publicrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleLivez is GET /livez: 200 while the process can serve HTTP. It checks no dependencies on purpose, so a
// liveness probe never restarts healthy pods during a MongoDB outage (readiness uses /health for that).
func HandleLivez(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
