package request

import (
	"strconv"
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

// ListChatMessagesReq is GET /chat/sessions/:sid/messages?before&limit.
type ListChatMessagesReq struct {
	SessionID string
	// RawBefore and RawLimit are the query values; Validate parses them into Before and Limit.
	RawBefore, RawLimit string
	Before, Limit       int
}

// Validate parses the paging query: before is a message idx, limit is 1-50 (default 30).
func (r *ListChatMessagesReq) Validate() apperrors.IHttpError {
	r.Limit = constants.DefaultChatPageLimit
	if r.RawLimit != "" {
		n, err := strconv.Atoi(r.RawLimit)
		if err != nil || n < 1 || n > constants.MaxChatPageLimit {
			return apperrors.BadRequestError("limit must be 1-50")
		}
		r.Limit = n
	}
	if r.RawBefore != "" {
		n, err := strconv.Atoi(r.RawBefore)
		if err != nil || n < 1 {
			return apperrors.BadRequestError("before must be a positive message idx")
		}
		r.Before = n
	}
	return nil
}
