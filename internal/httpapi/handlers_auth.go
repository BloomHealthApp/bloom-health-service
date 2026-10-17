package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"time"

	"github.com/bubu-bloom/service/internal/auth"
	"github.com/bubu-bloom/service/internal/model"
	"github.com/bubu-bloom/service/internal/store"
)

func (s *Server) userDTO(ctx context.Context, u *model.User, m *model.Membership) map[string]any {
	ownerName := u.DisplayName
	if members, err := s.Store.MembersOfSpace(ctx, m.SpaceID); err == nil {
		for _, mm := range members {
			if mm.Role == model.RoleOwner {
				if ou, err := s.Store.UserByID(ctx, mm.UserID); err == nil {
					ownerName = ou.DisplayName
				}
			}
		}
	}
	var avatar any
	if u.Avatar != nil {
		avatar = *u.Avatar
	}
	return map[string]any{"id": u.ID, "email": u.Email, "displayName": u.DisplayName, "avatar": avatar,
		"timezone": u.Timezone, "theme": u.Theme, "role": string(m.Role), "sharedSpaceId": m.SpaceID, "ownerDisplayName": ownerName}
}

type emailBody struct {
	Email string `json:"email"`
}

func (s *Server) otpRequest(w http.ResponseWriter, r *http.Request) {
	var b emailBody
	if !decode(w, r, &b) {
		return
	}
	email := auth.NormalizeEmail(b.Email)
	if _, err := mail.ParseAddress(email); err != nil || len(email) > 254 {
		writeErr(w, http.StatusBadRequest, "validation_failed", "a valid email is required", nil)
		return
	}
	if !s.emailLimit.allow(email) {
		// same response as success: do not reveal anything
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
		return
	}
	if err := s.Auth.RequestOTP(r.Context(), email); err != nil {
		s.Log.Error("otp request failed", "err", err.Error())
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

func (s *Server) tokenResponse(w http.ResponseWriter, r *http.Request, t *auth.Tokens, u *model.User) {
	m, err := s.Store.MembershipForUser(r.Context(), u.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "forbidden", "not a member of a shared space", nil)
			return
		}
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accessToken": t.AccessToken, "refreshToken": t.RefreshToken,
		"expiresIn": t.ExpiresIn, "user": s.userDTO(r.Context(), u, m)})
}

func (s *Server) otpVerify(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !s.emailLimit.allow("verify|" + auth.NormalizeEmail(b.Email)) {
		w.Header().Set("Retry-After", "60")
		writeErr(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later", nil)
		return
	}
	t, u, err := s.Auth.VerifyOTP(r.Context(), b.Email, b.Code)
	if err != nil {
		if errors.Is(err, auth.ErrInvalid) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid or expired code", nil)
			return
		}
		s.internal(w, r, err)
		return
	}
	s.tokenResponse(w, r, t, u)
}

type refreshBody struct {
	RefreshToken string `json:"refreshToken"`
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var b refreshBody
	if !decode(w, r, &b) {
		return
	}
	t, u, err := s.Auth.Refresh(r.Context(), b.RefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrInvalid) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token", nil)
			return
		}
		s.internal(w, r, err)
		return
	}
	s.tokenResponse(w, r, t, u)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var b refreshBody
	if !decode(w, r, &b) {
		return
	}
	if err := s.Auth.Logout(r.Context(), b.RefreshToken); err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	a := actorFrom(r)
	writeJSON(w, 200, s.userDTO(r.Context(), a.User, a.Membership))
}

var themes = map[string]bool{"cottonCandy": true, "lavenderDream": true, "mintPeach": true}

func (s *Server) patchMe(w http.ResponseWriter, r *http.Request) {
	a := actorFrom(r)
	var body map[string]any
	if !decode(w, r, &body) {
		return
	}
	u := *a.User
	for k, v := range body {
		switch k {
		case "displayName":
			str, ok := v.(string)
			if !ok || len([]rune(str)) < 1 || len([]rune(str)) > 40 {
				writeErr(w, 400, "validation_failed", "displayName must be 1-40 characters", nil)
				return
			}
			u.DisplayName = str
		case "avatar":
			if v == nil {
				u.Avatar = nil
				continue
			}
			str, ok := v.(string)
			if !ok || len(str) > 200 {
				writeErr(w, 400, "validation_failed", "avatar must be a short string or null", nil)
				return
			}
			u.Avatar = &str
		case "timezone":
			str, ok := v.(string)
			if _, err := time.LoadLocation(str); !ok || str == "" || str == "Local" || err != nil {
				writeErr(w, 400, "validation_failed", "timezone must be a valid IANA zone", nil)
				return
			}
			u.Timezone = str
		case "theme":
			str, ok := v.(string)
			if !ok || !themes[str] {
				writeErr(w, 400, "validation_failed", "invalid theme", nil)
				return
			}
			u.Theme = str
		default:
			writeErr(w, 400, "validation_failed", "unknown field "+k, nil)
			return
		}
	}
	u.UpdatedAt = time.Now().UTC()
	if err := s.Store.SaveUser(r.Context(), &u); err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, s.userDTO(r.Context(), &u, a.Membership))
}
