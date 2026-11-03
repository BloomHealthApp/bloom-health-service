package request

import (
	"strconv"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

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
