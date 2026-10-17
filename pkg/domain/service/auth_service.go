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
	"math"
	"math/big"
	"strconv"
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

// IAuthService implements email-OTP sign-up and sign-in (no passwords), short-lived JWT access tokens
// and rotating refresh tokens.
type IAuthService interface {
	// RequestSignUp emails a code to req.Email and returns the challenge to verify. req must already be
	// validated and normalized (E.164 phone, lower-case email). 409 already_registered when the email or
	// phone belongs to an account; 429 during the per-account resend cooldown. No account is created yet.
	RequestSignUp(ctx context.Context, req *request.SignUpReq) (*response.ChallengeResponse, apperrors.IHttpError)
	// RequestSignIn looks the account up by email or phone and mails the code to the email on file.
	// It always returns a challenge: for an unknown identifier (or an internal failure) the challengeId
	// is a random decoy that can never verify and nothing is sent, so accounts cannot be enumerated.
	RequestSignIn(ctx context.Context, req *request.SignInReq) (*response.ChallengeResponse, apperrors.IHttpError)
	// VerifyOTP consumes the challenge (single use). A sign-up challenge first creates the user, private
	// space, default habits and settings from the pending name/phone/email.
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
	provision   IProvisionService
	defaultTZ   string
	clock       clock.Clock
	secret      []byte
	issuer      string
	resendAfter time.Duration
}

// NewAuthService wires the auth service.
func NewAuthService(log logger.ILogger, cfg *config.Store, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo,
	otps mongorepo.IOTPRepo, sessions mongorepo.ISessionRepo, mailer smtp.IMailer, provision IProvisionService, clk clock.Clock) IAuthService {
	return &authService{log: log, users: users, memberships: memberships, otps: otps, sessions: sessions, mailer: mailer,
		provision: provision, defaultTZ: cfg.DefaultTZ, clock: clk, secret: []byte(cfg.AccessSecret), issuer: cfg.Issuer, resendAfter: cfg.OTPResendAfter}
}

func internalErr(log logger.ILogger, ctx context.Context, msg string, err error) apperrors.IHttpError {
	log.Error(ctx, msg, map[string]any{"err": err.Error()})
	return apperrors.InternalServerError("something went wrong")
}

func (s *authService) otpHash(challengeID, code string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte("otp|" + challengeID + "|" + code))
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

// cooldown returns the newest challenge mailed to email while it is still inside the resend window.
func (s *authService) cooldown(ctx context.Context, email string) (*dao.OTPChallenge, time.Duration) {
	cur, err := s.otps.LatestByEmail(ctx, email)
	if err != nil {
		return nil, 0
	}
	if wait := cur.CreatedAt.Add(s.resendAfter).Sub(s.clock.Now()); wait > 0 {
		return cur, wait
	}
	return nil, 0
}

// sendChallenge stores a new challenge for email, retires the previous one and mails the code.
func (s *authService) sendChallenge(ctx context.Context, purpose, email, name, phone, avatarID string) (string, error) {
	id, err := randomToken()
	if err != nil {
		return "", err
	}
	code, err := randomCode()
	if err != nil {
		return "", err
	}
	prev, _ := s.otps.LatestByEmail(ctx, email)
	now := s.clock.Now()
	ch := &dao.OTPChallenge{ChallengeID: id, Purpose: purpose, Email: email, PendingName: name, PendingPhone: phone, PendingAvatar: avatarID,
		CodeHash: s.otpHash(id, code), CreatedAt: now, ExpiresAt: now.Add(constants.OTPTTL)}
	if err := s.otps.Save(ctx, ch); err != nil {
		return "", fmt.Errorf("save challenge: %w", err)
	}
	if prev != nil && prev.ChallengeID != id {
		_ = s.otps.Delete(ctx, prev.ChallengeID) // only the newest code stays valid
	}
	kind := smtp.OTPSignIn
	if purpose == dao.OTPPurposeSignUp {
		kind = smtp.OTPSignUp
	}
	subject, text, html := smtp.OTPEmail(kind, name, code, int(constants.OTPTTL.Minutes()))
	if err := s.mailer.Send(ctx, email, subject, text, html); err != nil {
		return "", fmt.Errorf("send mail: %w", err)
	}
	return id, nil
}

func (s *authService) RequestSignUp(ctx context.Context, req *request.SignUpReq) (*response.ChallengeResponse, apperrors.IHttpError) {
	taken := apperrors.AlreadyRegisteredError("an account with this email or phone already exists")
	if _, err := s.users.ByEmail(ctx, req.Email); err == nil {
		return nil, taken
	} else if !apperrors.IsNotFound(err) {
		return nil, internalErr(s.log, ctx, "signup: email lookup failed", err)
	}
	if _, err := s.users.ByPhone(ctx, req.Phone); err == nil {
		return nil, taken
	} else if !apperrors.IsNotFound(err) {
		return nil, internalErr(s.log, ctx, "signup: phone lookup failed", err)
	}
	if cur, wait := s.cooldown(ctx, req.Email); cur != nil {
		return nil, apperrors.TooManyRequestsError("a code was just sent, please wait before requesting another",
			strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	}
	id, err := s.sendChallenge(ctx, dao.OTPPurposeSignUp, req.Email, req.Name, req.Phone, deref(req.Avatar))
	if err != nil {
		return nil, internalErr(s.log, ctx, "signup: could not issue code", err)
	}
	return &response.ChallengeResponse{Status: "sent", ChallengeID: id}, nil
}

func (s *authService) RequestSignIn(ctx context.Context, req *request.SignInReq) (*response.ChallengeResponse, apperrors.IHttpError) {
	decoy := func() (*response.ChallengeResponse, apperrors.IHttpError) {
		id, err := randomToken()
		if err != nil {
			return nil, internalErr(s.log, ctx, "signin: decoy generation failed", err)
		}
		return &response.ChallengeResponse{Status: "sent", ChallengeID: id}, nil
	}
	var (
		u   *dao.User
		err error
	)
	if req.IsEmail {
		u, err = s.users.ByEmail(ctx, req.Identifier)
	} else {
		u, err = s.users.ByPhone(ctx, req.Identifier)
	}
	if err != nil {
		if !apperrors.IsNotFound(err) {
			s.log.Error(ctx, "signin: lookup failed", map[string]any{"err": err.Error()})
		}
		return decoy()
	}
	email := request.NormalizeEmail(u.Email)
	if cur, _ := s.cooldown(ctx, email); cur != nil && cur.Purpose == dao.OTPPurposeSignIn {
		return &response.ChallengeResponse{Status: "sent", ChallengeID: cur.ChallengeID}, nil // code already mailed
	}
	id, err := s.sendChallenge(ctx, dao.OTPPurposeSignIn, email, "", "", "")
	if err != nil {
		s.log.Error(ctx, "signin: could not issue code", map[string]any{"err": err.Error()})
		return decoy() // never reveal the failure: the caller sees the same answer as for unknown accounts
	}
	return &response.ChallengeResponse{Status: "sent", ChallengeID: id}, nil
}

func (s *authService) VerifyOTP(ctx context.Context, req *request.OTPVerifyReq) (*response.AuthResponse, apperrors.IHttpError) {
	invalid := apperrors.UnauthorizedError("invalid or expired code")
	if req.ChallengeID == "" {
		return nil, invalid
	}
	ch, err := s.otps.Get(ctx, req.ChallengeID)
	if err != nil {
		return nil, invalid
	}
	now := s.clock.Now()
	if now.After(ch.ExpiresAt) || ch.Attempts >= constants.OTPMaxAttempts {
		_ = s.otps.Delete(ctx, ch.ChallengeID)
		return nil, invalid
	}
	if subtle.ConstantTimeCompare([]byte(s.otpHash(ch.ChallengeID, req.Code)), []byte(ch.CodeHash)) != 1 {
		if n, _ := s.otps.AddAttempt(ctx, ch.ChallengeID); n >= constants.OTPMaxAttempts {
			_ = s.otps.Delete(ctx, ch.ChallengeID)
		}
		return nil, invalid
	}
	if err := s.otps.Delete(ctx, ch.ChallengeID); err != nil {
		return nil, internalErr(s.log, ctx, "otp: delete failed", err)
	}
	var u *dao.User
	switch ch.Purpose {
	case dao.OTPPurposeSignUp:
		u, err = s.provision.EnsureAccount(ctx, ch.PendingName, ch.PendingPhone, ch.PendingAvatar, ch.Email, s.defaultTZ)
		if apperrors.IsConflict(err) { // phone (or email) taken between request and verify: unique-index race
			return nil, apperrors.AlreadyRegisteredError("an account with this email or phone already exists")
		}
		if err != nil {
			return nil, internalErr(s.log, ctx, "otp: account provisioning failed", err)
		}
	case dao.OTPPurposeSignIn:
		u, err = s.users.ByEmail(ctx, ch.Email)
		if apperrors.IsNotFound(err) {
			return nil, invalid
		}
		if err != nil {
			return nil, internalErr(s.log, ctx, "otp: user lookup failed", err)
		}
	default:
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
