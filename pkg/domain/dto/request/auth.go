package request

import (
	"net/mail"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/avatar"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/gender"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func validEmail(e string) bool {
	_, err := mail.ParseAddress(e)
	return err == nil && len(e) <= 254 && !strings.ContainsAny(e, " <>")
}

const phoneMsg = "phone must be a valid number (8-15 digits, optional +country code)"

// SignUpReq is POST /auth/signup/request.
type SignUpReq struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
	// Avatar is optional: absent or null means no avatar, otherwise an id from pkg/enums/avatar.
	Avatar *string `json:"avatar"`
	// Gender is optional: absent or null means not provided, otherwise an id from pkg/enums/gender.
	Gender *string `json:"gender"`
}

// Validate trims and checks name and email and requires a phone; Normalize canonicalizes the phone.
func (r *SignUpReq) Validate() apperrors.IHttpError {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = NormalizeEmail(r.Email)
	r.Phone = strings.TrimSpace(r.Phone)
	return toHTTPError(validation.ValidateStruct(r,
		validation.Field(&r.Name, rule("name must be 1-40 characters", func(v any) bool {
			n := len([]rune(v.(string)))
			return n >= 1 && n <= 40
		})),
		validation.Field(&r.Phone, rule(phoneMsg, func(v any) bool { return v.(string) != "" })),
		validation.Field(&r.Email, rule("a valid email is required", func(v any) bool { return validEmail(v.(string)) })),
		validation.Field(&r.Avatar, rule("avatar must be null or one of bubu-01 to bubu-09", func(v any) bool {
			a, _ := v.(*string)
			return a == nil || avatar.IsValid(*a)
		})),
		validation.Field(&r.Gender, rule("gender must be null or one of female, male, other, prefer_not_to_say", func(v any) bool {
			g, _ := v.(*string)
			return g == nil || gender.Valid(*g)
		}))))
}

// Normalize rewrites Phone to E.164 using the default country code for bare 10-digit numbers.
func (r *SignUpReq) Normalize(defaultCC string) apperrors.IHttpError {
	p, err := utils.NormalizePhone(r.Phone, defaultCC)
	if err != nil {
		return apperrors.BadRequestError(phoneMsg)
	}
	r.Phone = p
	return nil
}

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

// OTPVerifyReq is POST /auth/otp/verify. Bad input is answered with the same 401 as a wrong code.
type OTPVerifyReq struct {
	ChallengeID string `json:"challengeId"`
	Code        string `json:"code"`
}

// Validate trims the fields.
func (r *OTPVerifyReq) Validate() apperrors.IHttpError {
	r.ChallengeID = strings.TrimSpace(r.ChallengeID)
	r.Code = strings.TrimSpace(r.Code)
	return nil
}

// RefreshTokenReq is POST /auth/refresh and POST /auth/logout.
type RefreshTokenReq struct {
	RefreshToken string `json:"refreshToken"`
}

// Validate is a no-op: an unknown token is answered by the service.
func (r *RefreshTokenReq) Validate() apperrors.IHttpError { return nil }
