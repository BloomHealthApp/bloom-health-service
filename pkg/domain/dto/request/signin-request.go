package request

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// SignInReq is POST /auth/signin/request. Identifier is an email (contains "@") or a phone number.
type SignInReq struct {
	Identifier string `json:"identifier"`
	// IsEmail is set by Normalize.
	IsEmail bool `json:"-"`
}

// Validate trims the identifier and checks it is present and plausible.
func (r *SignInReq) Validate() apperrors.IHttpError {
	r.Identifier = strings.TrimSpace(r.Identifier)
	return toHTTPError(validation.ValidateStruct(r,
		validation.Field(&r.Identifier, rule("identifier must be an email or a phone number", func(v any) bool {
			s := v.(string)
			return s != "" && len(s) <= 254
		}))))
}

// Normalize canonicalizes Identifier (lower-cased email or E.164 phone) and sets IsEmail.
func (r *SignInReq) Normalize(defaultCC string) apperrors.IHttpError {
	if strings.Contains(r.Identifier, "@") {
		e := NormalizeEmail(r.Identifier)
		if !validEmail(e) {
			return apperrors.BadRequestError("identifier must be an email or a phone number")
		}
		r.Identifier, r.IsEmail = e, true
		return nil
	}
	p, err := utils.NormalizePhone(r.Identifier, defaultCC)
	if err != nil {
		return apperrors.BadRequestError("identifier must be an email or a phone number")
	}
	r.Identifier, r.IsEmail = p, false
	return nil
}
