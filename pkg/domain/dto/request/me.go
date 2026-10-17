package request

import (
	"bytes"
	"encoding/json"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/bubu-bloom/service/pkg/enums/theme"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
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

// PatchMeReq is PATCH /me. Unknown fields (role, userId, ...) are rejected by strict binding.
type PatchMeReq struct {
	DisplayName OptString `json:"displayName"`
	Avatar      OptString `json:"avatar"`
	Timezone    OptString `json:"timezone"`
	Theme       OptString `json:"theme"`
}

// Validate checks every supplied field.
func (r *PatchMeReq) Validate() apperrors.IHttpError {
	return toHTTPError(validation.Errors{
		"displayName": validation.Validate(r.DisplayName, rule("displayName must be 1-40 characters", func(v any) bool {
			o := v.(OptString)
			n := len([]rune(o.Value))
			return !o.Set || (!o.Null && n >= 1 && n <= 40)
		})),
		"avatar": validation.Validate(r.Avatar, rule("avatar must be a short string or null", func(v any) bool {
			o := v.(OptString)
			return !o.Set || o.Null || len(o.Value) <= 200
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
