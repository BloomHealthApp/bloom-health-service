package request

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// SendChatMessageReq is POST /chat/sessions/:sid/messages.
type SendChatMessageReq struct {
	SessionID string  `json:"-"`
	Content   string  `json:"content"`
	ModelID   *string `json:"modelId"`
}

// Validate trims the content and checks its length.
func (r *SendChatMessageReq) Validate() apperrors.IHttpError {
	r.Content = strings.TrimSpace(r.Content)
	return toHTTPError(validation.Errors{
		"content": validation.Validate(r.Content, rule("content must be 1-4000 characters", func(v any) bool {
			n := len([]rune(v.(string)))
			return n >= 1 && n <= constants.MaxChatContentChars
		})),
	}.Filter())
}
