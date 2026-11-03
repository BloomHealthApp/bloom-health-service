package request

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/avatar"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/gender"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

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
		validation.Field(&r.Avatar, rule("avatar must be null or one of bubu-01 to bubu-15", func(v any) bool {
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
