package request

import (
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// RefreshTokenReq is POST /auth/refresh and POST /auth/logout.
type RefreshTokenReq struct {
	RefreshToken string `json:"refreshToken"`
}

// Validate is a no-op: an unknown token is answered by the service.
func (r *RefreshTokenReq) Validate() apperrors.IHttpError { return nil }
