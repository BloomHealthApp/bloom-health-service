package publicrouter

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/assets"
)

var deleteAccountPage = sync.OnceValue(func() string { return renderPolicyPage(assets.DeleteAccount) })

// HandleGetDeleteAccount is GET /delete-account: the public page that tells people how to delete their account and
// what is erased, linked from Google Play's Data safety section.
func HandleGetDeleteAccount(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Security-Policy", privacyCSP)
	h.Set("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(deleteAccountPage()))
}
