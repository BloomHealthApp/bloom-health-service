package request

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/avatar"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/gender"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/theme"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// PatchMeReq is PATCH /me. Unknown fields (role, userId, ...) are rejected by strict binding.
type PatchMeReq struct {
	DisplayName OptString `json:"displayName"`
	Avatar      OptString `json:"avatar"`
	Timezone    OptString `json:"timezone"`
	Theme       OptString `json:"theme"`
	// Gender is null or one of pkg/enums/gender.
	Gender OptString `json:"gender"`
	// PeriodTrackerEnabled is a bool, or null to reset to the gender default.
	PeriodTrackerEnabled OptBool `json:"periodTrackerEnabled"`
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
