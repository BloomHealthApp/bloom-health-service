// Package middleware holds the gin middleware chain: request info, recovery, secure headers, CORS,
// request logging, body limit, rate limiting and authentication.
package middleware

import (
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// GenericRequestInfo assigns a request id and copies optional client headers (device id, client
// type, app version, timezone) into the request context. Nothing here is trusted for authorization.
func GenericRequestInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.GetHeader(constants.HeaderRequestID)
		if !safeID.MatchString(id) {
			id = uuid.NewString()
		}
		c.Header(constants.HeaderRequestID, id)
		ctx = utils.SetInContext(ctx, utils.RequestIDKey, id)
		for key, header := range map[utils.CtxKey]string{
			utils.DeviceIDKey:   constants.HeaderDeviceID,
			utils.ClientTypeKey: constants.HeaderClientType,
			utils.AppVersionKey: constants.HeaderAppVersion,
			utils.TimezoneKey:   constants.HeaderTimezone,
		} {
			if v := c.GetHeader(header); v != "" && len(v) <= 128 {
				ctx = utils.SetInContext(ctx, key, v)
			}
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
