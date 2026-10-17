package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/constants"
	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/dto/response"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/infrastructure/transport/smtp"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils/clock"
)

// IAuthService implements email OTP login, short-lived JWT access tokens and rotating refresh tokens.
type IAuthService interface {
	// RequestOTP emails a code if (and only if) the email belongs to a provisioned user. It never
	// reveals whether the email is known: unknown emails and cooldowns return nil.
	RequestOTP(ctx context.Context, req *request.OTPRequestReq) apperrors.IHttpError
	VerifyOTP(ctx context.Context, req *request.OTPVerifyReq) (*response.AuthResponse, apperrors.IHttpError)
	// Refresh rotates a refresh token. Reusing an already-rotated token revokes the whole family.
	Refresh(ctx context.Context, req *request.RefreshTokenReq) (*response.AuthResponse, apperrors.IHttpError)
	Logout(ctx context.Context, req *request.RefreshTokenReq) apperrors.IHttpError
	// Authenticate verifies an access token and resolves user and membership from the database.
	// Roles are never read from the client or the token.
	Authenticate(ctx context.Context, accessToken string) (*serviceBos.Actor, apperrors.IHttpError)
}

type authService struct {
	log         logger.ILogger
	users       mongorepo.IUserRepo
	memberships mongorepo.IMembershipRepo
	otps        mongorepo.IOTPRepo
	sessions    mongorepo.ISessionRepo
	mailer      smtp.IMailer
	clock       clock.Clock
	secret      []byte
	issuer      string
	resendAfter time.Duration
}

// NewAuthService wires the auth service.
func NewAuthService(log logger.ILogger, cfg *config.Store, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo,
	otps mongorepo.IOTPRepo, sessions mongorepo.ISessionRepo, mailer smtp.IMailer, clk clock.Clock) IAuthService {
	return &authService{log: log, users: users, memberships: memberships, otps: otps, sessions: sessions, mailer: mailer,
		clock: clk, secret: []byte(cfg.AccessSecret), issuer: cfg.Issuer, resendAfter: cfg.OTPResendAfter}
}

func internalErr(log logger.ILogger, ctx context.Context, msg string, err error) apperrors.IHttpError {
	log.Error(ctx, msg, map[string]any{"err": err.Error()})
	return apperrors.InternalServerError("something went wrong")
}

func (s *authService) otpHash(email, code string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte("otp|" + email + "|" + code))
	return hex.EncodeToString(h.Sum(nil))
}

func sha(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *authService) RequestOTP(ctx context.Context, req *request.OTPRequestReq) apperrors.IHttpError {
	email := req.Email
	u, err := s.users.ByEmail(ctx, email)
	if apperrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return internalErr(s.log, ctx, "otp: user lookup failed", err)
	}
	now := s.clock.Now()
	if cur, err := s.otps.Get(ctx, email); err == nil {
		issued := cur.ExpiresAt.Add(-constants.OTPTTL)
		if now.Before(issued.Add(s.resendAfter)) {
			return nil // cooldown: silently skip
		}
	}
	code, err := randomCode()
	if err != nil {
		return internalErr(s.log, ctx, "otp: code generation failed", err)
	}
	if err := s.otps.Save(ctx, &dao.OTPChallenge{Email: email, CodeHash: s.otpHash(email, code), ExpiresAt: now.Add(constants.OTPTTL)}); err != nil {
		return internalErr(s.log, ctx, "otp: save failed", err)
	}
	body := fmt.Sprintf("Your Bubu Bloom sign-in code is %s. It expires in %d minutes. If you didn't ask for it, ignore this email.",
		code, int(constants.OTPTTL.Minutes()))
	if err := s.mailer.Send(ctx, u.Email, "Your Bubu Bloom code", body); err != nil {
		return internalErr(s.log, ctx, "otp: mail send failed", err)
	}
	return nil
}

func (s *authService) VerifyOTP(ctx context.Context, req *request.OTPVerifyReq) (*response.AuthResponse, apperrors.IHttpError) {
	invalid := apperrors.UnauthorizedError("invalid or expired code")
	email := req.Email
	ch, err := s.otps.Get(ctx, email)
	if err != nil {
		return nil, invalid
	}
	now := s.clock.Now()
	if now.After(ch.ExpiresAt) || ch.Attempts >= constants.OTPMaxAttempts {
		_ = s.otps.Delete(ctx, email)
		return nil, invalid
	}
	if subtle.ConstantTimeCompare([]byte(s.otpHash(email, req.Code)), []byte(ch.CodeHash)) != 1 {
		if n, _ := s.otps.AddAttempt(ctx, email); n >= constants.OTPMaxAttempts {
			_ = s.otps.Delete(ctx, email)
		}
		return nil, invalid
	}
	if err := s.otps.Delete(ctx, email); err != nil {
		return nil, internalErr(s.log, ctx, "otp: delete failed", err)
	}
	u, err := s.users.ByEmail(ctx, email)
	if err != nil {
		return nil, invalid
	}
	return s.issue(ctx, u, uuid.NewString())
}

// issue mints an access/refresh pair for a new session in the given family and renders the response.
func (s *authService) issue(ctx context.Context, u *dao.User, family string) (*response.AuthResponse, apperrors.IHttpError) {
	now := s.clock.Now()
	claims := jwt.RegisteredClaims{Subject: u.ID, Issuer: s.issuer, IssuedAt: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(constants.AccessTokenTTL)), ID: uuid.NewString()}
	at, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return nil, internalErr(s.log, ctx, "token signing failed", err)
	}
	rt, err := randomToken()
	if err != nil {
		return nil, internalErr(s.log, ctx, "refresh token generation failed", err)
	}
	err = s.sessions.Save(ctx, &dao.RefreshSession{ID: uuid.NewString(), FamilyID: family, UserID: u.ID,
		TokenHash: sha(rt), ExpiresAt: now.Add(constants.RefreshTokenTTL), CreatedAt: now})
	if err != nil {
		return nil, internalErr(s.log, ctx, "session save failed", err)
	}
	m, err := s.memberships.ForUser(ctx, u.ID)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.ForbiddenError("not a member of a shared space")
		}
		return nil, internalErr(s.log, ctx, "membership lookup failed", err)
	}
	return &response.AuthResponse{AccessToken: at, RefreshToken: rt, ExpiresIn: int(constants.AccessTokenTTL.Seconds()),
		User: buildUserResponse(ctx, s.users, s.memberships, u, m)}, nil
}

func (s *authService) Refresh(ctx context.Context, req *request.RefreshTokenReq) (*response.AuthResponse, apperrors.IHttpError) {
	invalid := apperrors.UnauthorizedError("invalid refresh token")
	sess, err := s.sessions.ByHash(ctx, sha(req.RefreshToken))
	if err != nil {
		return nil, invalid
	}
	if sess.RevokedAt != nil || s.clock.Now().After(sess.ExpiresAt) {
		return nil, invalid
	}
	if sess.UsedAt != nil {
		_ = s.sessions.RevokeFamily(ctx, sess.FamilyID)
		return nil, invalid
	}
	ok, err := s.sessions.MarkUsed(ctx, sess.ID)
	if err != nil {
		return nil, internalErr(s.log, ctx, "session rotate failed", err)
	}
	if !ok {
		_ = s.sessions.RevokeFamily(ctx, sess.FamilyID)
		return nil, invalid
	}
	u, err := s.users.ByID(ctx, sess.UserID)
	if err != nil {
		return nil, invalid
	}
	return s.issue(ctx, u, sess.FamilyID)
}

func (s *authService) Logout(ctx context.Context, req *request.RefreshTokenReq) apperrors.IHttpError {
	sess, err := s.sessions.ByHash(ctx, sha(req.RefreshToken))
	if err != nil {
		return nil // unknown token: nothing to revoke, never reveal
	}
	if err := s.sessions.RevokeFamily(ctx, sess.FamilyID); err != nil {
		return internalErr(s.log, ctx, "logout failed", err)
	}
	return nil
}

func (s *authService) Authenticate(ctx context.Context, accessToken string) (*serviceBos.Actor, apperrors.IHttpError) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(accessToken, claims, func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(s.issuer), jwt.WithTimeFunc(s.clock.Now), jwt.WithExpirationRequired())
	if errors.Is(err, jwt.ErrTokenExpired) {
		return nil, apperrors.TokenExpiredError("access token expired")
	}
	if err != nil || claims.Subject == "" {
		return nil, apperrors.UnauthorizedError("invalid token")
	}
	u, err := s.users.ByID(ctx, claims.Subject)
	if err != nil {
		return nil, apperrors.UnauthorizedError("invalid token")
	}
	m, err := s.memberships.ForUser(ctx, claims.Subject)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.ForbiddenError("not a member of a shared space")
		}
		return nil, internalErr(s.log, ctx, "membership lookup failed", err)
	}
	return &serviceBos.Actor{User: u, Membership: m}, nil
}
