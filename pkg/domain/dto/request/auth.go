package request

import (
	"net/mail"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	apperrors "github.com/bubu-bloom/service/pkg/errors"
)

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// OTPRequestReq is POST /auth/otp/request.
type OTPRequestReq struct {
	Email string `json:"email"`
}

// Validate normalizes the email and checks its format.
func (r *OTPRequestReq) Validate() apperrors.IHttpError {
	r.Email = NormalizeEmail(r.Email)
	return toHTTPError(validation.ValidateStruct(r,
		validation.Field(&r.Email, rule("a valid email is required", func(v any) bool {
			e := v.(string)
			_, err := mail.ParseAddress(e)
			return err == nil && len(e) <= 254
		}))))
}

// OTPVerifyReq is POST /auth/otp/verify. Bad input is answered with the same 401 as a wrong code.
type OTPVerifyReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// Validate normalizes the email.
func (r *OTPVerifyReq) Validate() apperrors.IHttpError {
	r.Email = NormalizeEmail(r.Email)
	r.Code = strings.TrimSpace(r.Code)
	return nil
}

// RefreshTokenReq is POST /auth/refresh and POST /auth/logout.
type RefreshTokenReq struct {
	RefreshToken string `json:"refreshToken"`
}

// Validate is a no-op: an unknown token is answered by the service.
func (r *RefreshTokenReq) Validate() apperrors.IHttpError { return nil }
