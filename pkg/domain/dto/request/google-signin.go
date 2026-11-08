package request

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// MaxGoogleIDTokenLen bounds the ID token; Google's are about 1-2 KB.
const MaxGoogleIDTokenLen = 4096

// GoogleSignInReq is POST /auth/google: the ID token from the app's native Google sign-in.
type GoogleSignInReq struct {
	IDToken string `json:"idToken"`
}

// Validate trims the token and checks it is present and not oversized. The service checks the rest.
func (r *GoogleSignInReq) Validate() apperrors.IHttpError {
	r.IDToken = strings.TrimSpace(r.IDToken)
	return toHTTPError(validation.ValidateStruct(r,
		validation.Field(&r.IDToken, rule("idToken is required", func(v any) bool {
			s := v.(string)
			return s != "" && len(s) <= MaxGoogleIDTokenLen
		}))))
}
