package request

import (
	"strings"

	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// OTPVerifyReq is POST /auth/otp/verify. Bad input is answered with the same 401 as a wrong code.
type OTPVerifyReq struct {
	ChallengeID string `json:"challengeId"`
	Code        string `json:"code"`
}

// Validate trims the fields.
func (r *OTPVerifyReq) Validate() apperrors.IHttpError {
	r.ChallengeID = strings.TrimSpace(r.ChallengeID)
	r.Code = strings.TrimSpace(r.Code)
	return nil
}
