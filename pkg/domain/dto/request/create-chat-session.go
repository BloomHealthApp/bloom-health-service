package request

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// CreateChatSessionReq is POST /chat/sessions. ModelID is checked against the catalogue by the service.
type CreateChatSessionReq struct {
	Title         *string  `json:"title"`
	ModelID       *string  `json:"modelId"`
	ContextScopes []string `json:"contextScopes"`
}

// Validate trims the title and refuses context scopes, which no worker provider supports yet.
func (r *CreateChatSessionReq) Validate() apperrors.IHttpError {
	if r.Title != nil {
		t := strings.TrimSpace(*r.Title)
		r.Title = &t
	}
	return toHTTPError(validation.Errors{
		"title": validation.Validate(r.Title, rule("title must be 1-80 characters", func(v any) bool {
			t := v.(*string)
			n := 0
			if t != nil {
				n = len([]rune(*t))
			}
			return t == nil || (n >= 1 && n <= constants.MaxChatTitleChars)
		})),
		"contextScopes": validation.Validate(r.ContextScopes, rule("contextScopes is not supported yet", func(v any) bool {
			return len(v.([]string)) == 0
		})),
	}.Filter())
}
