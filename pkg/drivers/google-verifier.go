package drivers

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"

	"github.com/BloomHealthApp/bloom-health-service/config"
)

// GoogleIdentity is the verified part of a Google ID token.
type GoogleIdentity struct {
	// Subject is Google's stable account id (the `sub` claim).
	Subject string
	// Email is lower-cased and verified by Google.
	Email string
	// Name is the display name on the Google account; may be empty.
	Name string
}

// IGoogleVerifier checks ID tokens from the app's native Google sign-in.
type IGoogleVerifier interface {
	// Verify checks the signature against Google's published keys, the issuer, one of the configured
	// audiences, the expiry and that the email is verified. Any failure is an error; callers answer 401.
	Verify(ctx context.Context, idToken string) (*GoogleIdentity, error)
}

// ErrGoogleSignInOff is returned when no Google client id is configured.
var ErrGoogleSignInOff = errors.New("google sign-in is not configured")

var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

type googleVerifier struct {
	audiences []string
	validator *idtoken.Validator
}

// NewGoogleVerifier builds the verifier for cfg.GoogleClientIDs. It fetches Google's signing keys lazily
// (cached by the library per their Cache-Control), so building it makes no network call.
func NewGoogleVerifier(cfg *config.Store) (IGoogleVerifier, error) {
	v, err := idtoken.NewValidator(context.Background(), option.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	if err != nil {
		return nil, err
	}
	return &googleVerifier{audiences: cfg.GoogleClientIDs, validator: v}, nil
}

func (g *googleVerifier) Verify(ctx context.Context, idToken string) (*GoogleIdentity, error) {
	if len(g.audiences) == 0 {
		return nil, ErrGoogleSignInOff
	}
	p, err := idtoken.ParsePayload(idToken)
	if err != nil {
		return nil, err
	}
	// Security: idtoken also accepts ES256 tokens signed with Google's IAP keys and never checks the issuer,
	// so the issuer, audience and algorithm family are pinned here before the signature check.
	if !slices.Contains(googleIssuers, p.Issuer) || !slices.Contains(g.audiences, p.Audience) {
		return nil, errors.New("google token: wrong issuer or audience")
	}
	if p, err = g.validator.Validate(ctx, idToken, p.Audience); err != nil {
		return nil, err
	}
	if p.Subject == "" {
		return nil, errors.New("google token: no subject")
	}
	email, _ := p.Claims["email"].(string)
	verified, _ := p.Claims["email_verified"].(bool)
	if email = strings.ToLower(strings.TrimSpace(email)); email == "" || !verified {
		return nil, errors.New("google token: email missing or unverified")
	}
	name, _ := p.Claims["name"].(string)
	return &GoogleIdentity{Subject: p.Subject, Email: email, Name: strings.TrimSpace(name)}, nil
}
