package request

import (
	"bytes"
	"encoding/json"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/avatar"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/gender"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/theme"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// OptString distinguishes an absent field from an explicit JSON null.
type OptString struct {
	Set   bool
	Null  bool
	Value string
}

// UnmarshalJSON records presence; non-string, non-null values are decode errors (400).
func (o *OptString) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// OptBool distinguishes an absent field from an explicit JSON null.
type OptBool struct {
	Set   bool
	Null  bool
	Value bool
}

// UnmarshalJSON records presence; non-bool, non-null values are decode errors (400).
func (o *OptBool) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// PatchMeReq is PATCH /me. Unknown fields (role, userId, ...) are rejected by strict binding.
type PatchMeReq struct {
	DisplayName OptString `json:"displayName"`
	Avatar      OptString `json:"avatar"`
	Timezone    OptString `json:"timezone"`
	Theme       OptString `json:"theme"`
	Phone       OptString `json:"phone"`
	// Gender is null or one of pkg/enums/gender.
	Gender OptString `json:"gender"`
	// PeriodTrackerEnabled is a bool, or null to reset to the gender default.
	PeriodTrackerEnabled OptBool `json:"periodTrackerEnabled"`
}

// Normalize rewrites a supplied phone to E.164 (400 when it is not a valid number).
func (r *PatchMeReq) Normalize(defaultCC string) apperrors.IHttpError {
	if !r.Phone.Set {
		return nil
	}
	p, err := utils.NormalizePhone(r.Phone.Value, defaultCC)
	if err != nil {
		return apperrors.BadRequestError("phone must be a valid number (8-15 digits, optional +country code)")
	}
	r.Phone.Value = p
	return nil
}

// Validate checks every supplied field.
func (r *PatchMeReq) Validate() apperrors.IHttpError {
	return toHTTPError(validation.Errors{
		"displayName": validation.Validate(r.DisplayName, rule("displayName must be 1-40 characters", func(v any) bool {
			o := v.(OptString)
			n := len([]rune(o.Value))
			return !o.Set || (!o.Null && n >= 1 && n <= 40)
		})),
		"avatar": validation.Validate(r.Avatar, rule("avatar must be null or one of bubu-01 to bubu-15", func(v any) bool {
			o := v.(OptString)
			return !o.Set || o.Null || avatar.IsValid(o.Value)
		})),
		"gender": validation.Validate(r.Gender, rule("gender must be null or one of female, male, other, prefer_not_to_say", func(v any) bool {
			o := v.(OptString)
			return !o.Set || o.Null || gender.Valid(o.Value)
		})),
		"phone": validation.Validate(r.Phone, rule("phone must be a valid number (8-15 digits, optional +country code)", func(v any) bool {
			o := v.(OptString)
			return !o.Set || (!o.Null && o.Value != "")
		})),
		"theme": validation.Validate(r.Theme, rule("invalid theme", func(v any) bool {
			o := v.(OptString)
			return !o.Set || (!o.Null && theme.Valid(o.Value))
		})),
		"timezone": validation.Validate(r.Timezone, rule("timezone must be a valid IANA zone", func(v any) bool {
			o := v.(OptString)
			if !o.Set {
				return true
			}
			if o.Null || o.Value == "" || o.Value == "Local" {
				return false
			}
			_, err := time.LoadLocation(o.Value)
			return err == nil
		})),
	}.Filter())
}
