// Package auth implements email OTP login, short-lived JWT access tokens and rotating refresh tokens.
package auth

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
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/bubu-bloom/service/internal/mailer"
	"github.com/bubu-bloom/service/internal/model"
	"github.com/bubu-bloom/service/internal/store"
)

var (
	ErrInvalid = errors.New("invalid credentials")
	ErrExpired = errors.New("token expired")
)

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

type Service struct {
	Store       store.Store
	Mailer      mailer.Mailer
	Secret      []byte
	Issuer      string
	Now         func() time.Time
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	OTPTTL      time.Duration
	ResendAfter time.Duration
	MaxAttempts int
}

func New(st store.Store, m mailer.Mailer, secret, issuer string) *Service {
	return &Service{Store: st, Mailer: m, Secret: []byte(secret), Issuer: issuer, Now: time.Now,
		AccessTTL: 15 * time.Minute, RefreshTTL: 30 * 24 * time.Hour, OTPTTL: 10 * time.Minute,
		ResendAfter: 30 * time.Second, MaxAttempts: 5}
}

func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func (s *Service) otpHash(email, code string) string {
	h := hmac.New(sha256.New, s.Secret)
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

// RequestOTP emails a one-time code if (and only if) the email belongs to a provisioned user.
// It never reveals whether the email is known.
func (s *Service) RequestOTP(ctx context.Context, email string) error {
	email = NormalizeEmail(email)
	u, err := s.Store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := s.Now()
	if cur, err := s.Store.GetOTP(ctx, email); err == nil {
		issued := cur.ExpiresAt.Add(-s.OTPTTL)
		if now.Before(issued.Add(s.ResendAfter)) {
			return nil // cooldown: silently skip
		}
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	if err := s.Store.SaveOTP(ctx, &model.OTPChallenge{Email: email, CodeHash: s.otpHash(email, code), ExpiresAt: now.Add(s.OTPTTL)}); err != nil {
		return err
	}
	return s.Mailer.Send(ctx, u.Email, "Your Bubu Bloom code",
		fmt.Sprintf("Your Bubu Bloom sign-in code is %s. It expires in %d minutes. If you didn't ask for it, ignore this email.", code, int(s.OTPTTL.Minutes())))
}

func (s *Service) VerifyOTP(ctx context.Context, email, code string) (*Tokens, *model.User, error) {
	email = NormalizeEmail(email)
	ch, err := s.Store.GetOTP(ctx, email)
	if err != nil {
		return nil, nil, ErrInvalid
	}
	now := s.Now()
	if now.After(ch.ExpiresAt) || ch.Attempts >= s.MaxAttempts {
		_ = s.Store.DeleteOTP(ctx, email)
		return nil, nil, ErrInvalid
	}
	if subtle.ConstantTimeCompare([]byte(s.otpHash(email, strings.TrimSpace(code))), []byte(ch.CodeHash)) != 1 {
		if n, _ := s.Store.AddOTPAttempt(ctx, email); n >= s.MaxAttempts {
			_ = s.Store.DeleteOTP(ctx, email)
		}
		return nil, nil, ErrInvalid
	}
	if err := s.Store.DeleteOTP(ctx, email); err != nil {
		return nil, nil, err
	}
	u, err := s.Store.UserByEmail(ctx, email)
	if err != nil {
		return nil, nil, ErrInvalid
	}
	tok, err := s.issue(ctx, u.ID, uuid.NewString())
	return tok, u, err
}

func (s *Service) issue(ctx context.Context, userID, family string) (*Tokens, error) {
	now := s.Now()
	claims := jwt.RegisteredClaims{Subject: userID, Issuer: s.Issuer, IssuedAt: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.AccessTTL)), ID: uuid.NewString()}
	at, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.Secret)
	if err != nil {
		return nil, err
	}
	rt, err := randomToken()
	if err != nil {
		return nil, err
	}
	err = s.Store.SaveSession(ctx, &model.RefreshSession{ID: uuid.NewString(), FamilyID: family, UserID: userID,
		TokenHash: sha(rt), ExpiresAt: now.Add(s.RefreshTTL), CreatedAt: now})
	if err != nil {
		return nil, err
	}
	return &Tokens{AccessToken: at, RefreshToken: rt, ExpiresIn: int(s.AccessTTL.Seconds())}, nil
}

// Refresh rotates a refresh token. Reusing an already-rotated token revokes the whole family.
func (s *Service) Refresh(ctx context.Context, token string) (*Tokens, *model.User, error) {
	sess, err := s.Store.SessionByHash(ctx, sha(token))
	if err != nil {
		return nil, nil, ErrInvalid
	}
	if sess.RevokedAt != nil || s.Now().After(sess.ExpiresAt) {
		return nil, nil, ErrInvalid
	}
	if sess.UsedAt != nil {
		_ = s.Store.RevokeFamily(ctx, sess.FamilyID)
		return nil, nil, ErrInvalid
	}
	ok, err := s.Store.MarkSessionUsed(ctx, sess.ID)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		_ = s.Store.RevokeFamily(ctx, sess.FamilyID)
		return nil, nil, ErrInvalid
	}
	u, err := s.Store.UserByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, ErrInvalid
	}
	tok, err := s.issue(ctx, u.ID, sess.FamilyID)
	return tok, u, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	sess, err := s.Store.SessionByHash(ctx, sha(token))
	if err != nil {
		return nil
	}
	return s.Store.RevokeFamily(ctx, sess.FamilyID)
}

// ParseAccess validates an access token and returns the user id.
func (s *Service) ParseAccess(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return s.Secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(s.Issuer), jwt.WithTimeFunc(s.Now), jwt.WithExpirationRequired())
	if errors.Is(err, jwt.ErrTokenExpired) {
		return "", ErrExpired
	}
	if err != nil || claims.Subject == "" {
		return "", ErrInvalid
	}
	return claims.Subject, nil
}
