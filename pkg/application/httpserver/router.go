// Package httpserver builds the gin engine and its middleware chain.
package httpserver

import (
	"github.com/gin-gonic/gin"

	publicrouter "github.com/bubu-bloom/service/pkg/application/httpserver/v1/public-router"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/middleware"
	"github.com/bubu-bloom/service/pkg/utils"
)

// NewRouter builds the engine. It reads dependencies from di.AppLogic, which must be set first.
func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	_ = r.SetTrustedProxies(nil) // client IP handling is explicit (middleware.ClientIP)

	r.Use(
		middleware.GenericRequestInfo(),
		middleware.Recovery(),
		middleware.SecureHeaders(),
		middleware.CORS(),
		middleware.RequestLogger(),
		middleware.LimitIP(),
		middleware.BodyLimit(),
	)
	r.NoRoute(func(c *gin.Context) { utils.RespondError(c, apperrors.NotFoundError("no such route")) })
	r.NoMethod(func(c *gin.Context) { utils.RespondError(c, apperrors.MethodNotAllowedError("method not allowed")) })

	r.GET("/health", publicrouter.Health)
	publicrouter.RegisterRoutes(r.Group("/v1"))
	return r
}
