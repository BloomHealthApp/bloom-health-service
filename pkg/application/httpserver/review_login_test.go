package httpserver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

const (
	reviewEmail = "review@example.com"
	reviewCode  = "481937"
)

// withReviewLogin rebuilds the app on e's repositories with the review login turned on.
func withReviewLogin(t *testing.T, e *env) {
	t.Helper()
	cfg := &config.Store{ActiveEnv: "DEV", StoreKind: "memory", AccessSecret: secret, Issuer: issuer, Version: "1.0.0",
		Mailer: "console", ReviewEmail: reviewEmail, ReviewCode: reviewCode}
	app, err := di.InitializeTestApp(cfg, logger.NewDiscard(), e.clock, e.mail, e.google, e.repos)
	require.NoError(t, err)
	di.AppLogic = app
}

func TestReviewLoginUsesTheFixedCodeAfterARealSignUp(t *testing.T) {
	e := setup(t)
	withReviewLogin(t, e)

	// Sign-up is never shortcut: the code is mailed, so the account needs a real inbox once.
	e.signup("Play Reviewer", "9876500099", reviewEmail)

	e.mail.Messages = nil
	st, ch := e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": "Review@Example.com"})
	require.Equal(t, 202, st)
	require.Empty(t, e.mail.Messages, "no mail for the review account")

	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": ch["challengeId"], "code": "000001"})
	require.Equal(t, 401, st, "a wrong code still fails")
	st, body := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": ch["challengeId"], "code": reviewCode})
	require.Equal(t, 200, st, body)
	require.NotEmpty(t, body["accessToken"])

	// Signing in by the account's phone number works the same way.
	st, ch = e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": "9876500099"})
	require.Equal(t, 202, st)
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": ch["challengeId"], "code": reviewCode})
	require.Equal(t, 200, st)
}

func TestReviewLoginLeavesOtherAccountsAlone(t *testing.T) {
	e := setup(t)
	withReviewLogin(t, e)

	e.mail.Messages = nil
	st, ch := e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": "anu@example.com"})
	require.Equal(t, 202, st)
	require.Len(t, e.mail.Messages, 1, "everyone else still gets an emailed code")
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": ch["challengeId"], "code": reviewCode})
	require.Equal(t, 401, st, "the review code never works for another account")

	// Before the review account exists, signing in with its email gets the usual decoy and sends nothing.
	e.mail.Messages = nil
	st, ch = e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": reviewEmail})
	require.Equal(t, 202, st)
	require.Empty(t, e.mail.Messages)
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": ch["challengeId"], "code": reviewCode})
	require.Equal(t, 401, st)
}

func TestReviewLoginIsOffByDefault(t *testing.T) {
	e := setup(t)
	e.signup("Play Reviewer", "9876500099", reviewEmail)
	e.mail.Messages = nil
	st, ch := e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": reviewEmail})
	require.Equal(t, 202, st)
	require.Len(t, e.mail.Messages, 1)
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": ch["challengeId"], "code": reviewCode})
	require.Equal(t, 401, st)
}
