package httpserver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// fakeGoogle accepts the ID tokens registered in ids; anything else fails like a bad signature.
type fakeGoogle struct {
	ids map[string]*drivers.GoogleIdentity
}

func (f *fakeGoogle) Verify(_ context.Context, idToken string) (*drivers.GoogleIdentity, error) {
	if id, ok := f.ids[idToken]; ok {
		return id, nil
	}
	return nil, errors.New("bad token")
}

// withGoogle rebuilds the app on e's repositories with Google sign-in turned on.
func withGoogle(t *testing.T, e *env) {
	t.Helper()
	cfg := &config.Store{ActiveEnv: "DEV", StoreKind: "memory", AccessSecret: secret, Issuer: issuer, Version: "1.0.0",
		Mailer: "console", GoogleClientIDs: []string{"web-client.apps.googleusercontent.com"}}
	app, err := di.InitializeTestApp(cfg, logger.NewDiscard(), e.clock, e.mail, e.google, e.repos)
	require.NoError(t, err)
	di.AppLogic = app
}

func googleSignIn(e *env, token string) (int, map[string]any) {
	e.t.Helper()
	return e.do("POST", "/v1/auth/google", "", map[string]any{"idToken": token})
}

func TestGoogleSignInIsOffByDefault(t *testing.T) {
	e := setup(t)
	e.google.ids["tok"] = &drivers.GoogleIdentity{Subject: "g1", Email: "new@example.com", Name: "New"}
	st, _ := googleSignIn(e, "tok")
	require.Equal(t, 404, st)
}

func TestGoogleSignInCreatesAnAccountOnce(t *testing.T) {
	e := setup(t)
	withGoogle(t, e)
	e.google.ids["tok"] = &drivers.GoogleIdentity{Subject: "g1", Email: "new@example.com", Name: "Neha"}

	st, body := googleSignIn(e, "tok")
	require.Equal(t, 200, st, body)
	require.Equal(t, true, body["isNewUser"])
	user := body["user"].(map[string]any)
	require.Equal(t, "Neha", user["displayName"])
	require.Equal(t, "owner", user["role"])
	require.Nil(t, user["phone"], "Google accounts start without a phone")
	require.Empty(t, e.mail.Messages, "no code is mailed")

	st, habits := e.do("GET", "/v1/habits", body["accessToken"].(string), nil)
	require.Equal(t, 200, st)
	require.NotEmpty(t, habits["items"], "the new space has the default habits")

	st, again := googleSignIn(e, "tok")
	require.Equal(t, 200, st)
	require.Nil(t, again["isNewUser"])
	require.Equal(t, user["id"], again["user"].(map[string]any)["id"])
}

func TestGoogleSignInLinksAnExistingEmailAccount(t *testing.T) {
	e := setup(t)
	withGoogle(t, e)
	anu, err := e.repos.Users.ByEmail(context.Background(), "anu@example.com")
	require.NoError(t, err)
	e.google.ids["tok"] = &drivers.GoogleIdentity{Subject: "g-anu", Email: "anu@example.com", Name: "Someone Else"}

	st, body := googleSignIn(e, "tok")
	require.Equal(t, 200, st, body)
	require.Nil(t, body["isNewUser"])
	user := body["user"].(map[string]any)
	require.Equal(t, anu.ID, user["id"])
	require.Equal(t, "Anu", user["displayName"], "the profile is kept")

	stored, err := e.repos.Users.ByGoogleSub(context.Background(), "g-anu")
	require.NoError(t, err)
	require.Equal(t, anu.ID, stored.ID)

	// A different Google account with the same email can't take it over.
	e.google.ids["other"] = &drivers.GoogleIdentity{Subject: "g-other", Email: "anu@example.com"}
	st, body = googleSignIn(e, "other")
	require.Equal(t, 409, st)
	require.Equal(t, "already_registered", body["error"].(map[string]any)["code"])

	// Email codes still work for the linked account.
	access, _ := e.login("anu@example.com")
	require.NotEmpty(t, access)
}

func TestGoogleSignInRejectsBadInput(t *testing.T) {
	e := setup(t)
	withGoogle(t, e)

	st, _ := googleSignIn(e, "forged")
	require.Equal(t, 401, st)
	st, _ = googleSignIn(e, "  ")
	require.Equal(t, 400, st)
	st, _ = e.do("POST", "/v1/auth/google", "", map[string]any{"idToken": "x", "role": "owner"})
	require.Equal(t, 400, st, "unknown fields are rejected")
}
