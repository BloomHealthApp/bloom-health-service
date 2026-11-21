package request

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// SendConnectionReq is POST /connections: a friend request can only be made through someone's profile link code.
type SendConnectionReq struct {
	Code string `json:"code"`
}

// Validate checks the code's shape (base62, fixed length); whether it exists is the service's job.
func (r *SendConnectionReq) Validate() apperrors.IHttpError {
	return toHTTPError(validation.Errors{
		"code": validation.Validate(r.Code, rule("code is not a profile link code", func(v any) bool { return IsProfileLinkCode(v.(string)) })),
	}.Filter())
}

// IsProfileLinkCode reports whether s looks like a profile link code.
func IsProfileLinkCode(s string) bool {
	if len(s) != constants.ProfileLinkCodeLen {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}
