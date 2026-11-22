package request

import (
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// SendDMReq is POST /dm/:userId/messages. ID is chosen by the app, so resending after a lost reply is safe.
type SendDMReq struct {
	FriendID string `json:"-"`
	ID       string `json:"id"`
	Text     string `json:"text"`
}

// Validate trims the text and checks the id and length.
func (r *SendDMReq) Validate() apperrors.IHttpError {
	r.Text = strings.TrimSpace(r.Text)
	return toHTTPError(validation.Errors{
		"id": validation.Validate(r.ID, rule("id must be a UUID", func(v any) bool {
			_, err := uuid.Parse(v.(string))
			return err == nil
		})),
		"text": validation.Validate(r.Text, rule("text must be 1-1000 characters", func(v any) bool {
			n := len([]rune(v.(string)))
			return n >= 1 && n <= constants.MaxDMChars
		})),
	}.Filter())
}

// ListDMsReq is GET /dm/:userId/messages?before&after&limit. before and after are message seq numbers.
type ListDMsReq struct {
	FriendID                      string
	RawBefore, RawAfter, RawLimit string
	Before, After                 int64
	Limit                         int
}

// Validate parses the paging query: limit is 1-100 (default 30); before and after are exclusive.
func (r *ListDMsReq) Validate() apperrors.IHttpError {
	r.Limit = constants.DefaultDMPageLimit
	if r.RawLimit != "" {
		n, err := strconv.Atoi(r.RawLimit)
		if err != nil || n < 1 || n > constants.MaxDMPageLimit {
			return apperrors.BadRequestError("limit must be 1-100")
		}
		r.Limit = n
	}
	parse := func(raw, name string) (int64, apperrors.IHttpError) {
		if raw == "" {
			return 0, nil
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return 0, apperrors.BadRequestError(name + " must be a message seq")
		}
		return n, nil
	}
	var herr apperrors.IHttpError
	if r.Before, herr = parse(r.RawBefore, "before"); herr != nil {
		return herr
	}
	if r.After, herr = parse(r.RawAfter, "after"); herr != nil {
		return herr
	}
	if r.Before > 0 && r.After > 0 {
		return apperrors.BadRequestError("use before or after, not both")
	}
	return nil
}

// PushTokenReq is PUT and DELETE /me/push-token.
type PushTokenReq struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// Validate checks the token looks like an Expo push token. Platform is only needed to save one.
func (r *PushTokenReq) Validate(saving bool) apperrors.IHttpError {
	r.Token = strings.TrimSpace(r.Token)
	errs := validation.Errors{
		"token": validation.Validate(r.Token, rule("token must be an Expo push token", func(v any) bool {
			t := v.(string)
			return len(t) <= constants.MaxPushTokenChars && (strings.HasPrefix(t, "ExponentPushToken[") || strings.HasPrefix(t, "ExpoPushToken[")) && strings.HasSuffix(t, "]")
		})),
	}
	if saving {
		errs["platform"] = validation.Validate(r.Platform, validation.In("android", "ios").Error("platform must be android or ios"))
	}
	return toHTTPError(errs.Filter())
}
