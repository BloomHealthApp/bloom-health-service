package request

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// PhoneChangeReq is POST /me/phone/request.
type PhoneChangeReq struct {
	Phone string `json:"phone"`
}

// Validate trims the phone and requires it; Normalize canonicalizes it.
func (r *PhoneChangeReq) Validate() apperrors.IHttpError {
	r.Phone = strings.TrimSpace(r.Phone)
	return toHTTPError(validation.ValidateStruct(r,
		validation.Field(&r.Phone, rule(phoneMsg, func(v any) bool { return v.(string) != "" }))))
}

// Normalize rewrites the phone to E.164.
func (r *PhoneChangeReq) Normalize(defaultCC string) apperrors.IHttpError {
	p, err := utils.NormalizePhone(r.Phone, defaultCC)
	if err != nil {
		return apperrors.BadRequestError(phoneMsg)
	}
	r.Phone = p
	return nil
}
