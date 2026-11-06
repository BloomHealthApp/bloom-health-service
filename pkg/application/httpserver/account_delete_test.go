package httpserver_test

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

const cronSecret = "cron-secret-for-tests"

// withCron rebuilds the app on e's repositories with the daily purge endpoint turned on.
func withCron(t *testing.T, e *env) {
	t.Helper()
	cfg := &config.Store{ActiveEnv: "DEV", StoreKind: "memory", AccessSecret: secret, Issuer: issuer, Version: "1.0.0",
		Mailer: "console", CronSecret: cronSecret}
	app, err := di.InitializeTestApp(cfg, logger.NewDiscard(), e.clock, e.mail, e.repos)
	require.NoError(t, err)
	di.AppLogic = app
}

// purge calls the daily cron endpoint the way Vercel Cron does and returns the status and body.
func (e *env) purge(auth string) (int, string) {
	e.t.Helper()
	req, err := http.NewRequest("GET", e.srv.URL+"/internal/cron/purge-deleted-accounts", nil)
	require.NoError(e.t, err)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(e.t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (e *env) subjects() []string {
	var out []string
	for _, m := range e.mail.Messages {
		out = append(out, m.Subject)
	}
	return out
}

func TestDeleteMeLocksTheAccountUntilRestored(t *testing.T) {
	e := setup(t)
	anu, refresh := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")

	e.mail.Messages = nil
	st, body := e.do("DELETE", "/v1/me", anu, nil)
	require.Equal(t, 202, st, body)
	deleteAfter, err := time.Parse(time.RFC3339, body["deleteAfter"].(string))
	require.NoError(t, err)
	require.WithinDuration(t, e.clock.Now().Add(constants.AccountDeletionGrace), deleteAfter, time.Minute)
	require.Contains(t, e.subjects(), "Your Bloom Health account will be deleted")

	st, again := e.do("DELETE", "/v1/me", anu, nil)
	require.Equal(t, 202, st)
	require.Equal(t, body["deleteAfter"], again["deleteAfter"], "asking again keeps the same date")

	// Locked: only the profile (to show the date) and restore work, for the owner and the owner's viewer.
	st, me := e.do("GET", "/v1/me", anu, nil)
	require.Equal(t, 200, st)
	require.Equal(t, body["deleteAfter"], me["deletionScheduledAt"])
	for _, tok := range []string{anu, aman} {
		for _, path := range []string{"/v1/habits", "/v1/sync/pull", "/v1/chat/sessions"} {
			st, b := e.do("GET", path, tok, nil)
			require.Equal(t, 403, st, path)
			require.Equal(t, apperrors.CodeAccountPendingDeletion, b["error"].(map[string]any)["code"], path)
		}
	}
	st, _ = e.do("PUT", "/v1/logs/default-water/"+dayStr(0), anu, map[string]any{"value": 1, "baseVersion": 0})
	require.Equal(t, 403, st, "no writes while locked")

	st, _ = e.do("POST", "/v1/auth/refresh", "", map[string]any{"refreshToken": refresh})
	require.Equal(t, 401, st, "every device is signed out")

	// Signing in still works, so the person can restore.
	anu, _ = e.login("anu@example.com")
	e.mail.Messages = nil
	st, me = e.do("POST", "/v1/me/restore", anu, nil)
	require.Equal(t, 200, st, me)
	require.Nil(t, me["deletionScheduledAt"])
	require.Contains(t, e.subjects(), "Your Bloom Health account has been restored")
	for _, tok := range []string{anu, aman} {
		st, _ = e.do("GET", "/v1/habits", tok, nil)
		require.Equal(t, 200, st)
	}
	st, _ = e.do("POST", "/v1/me/restore", anu, nil)
	require.Equal(t, 409, st, "nothing to restore")
}

func TestPurgeErasesOwnerAndSpaceAfterTheGracePeriod(t *testing.T) {
	e := setup(t)
	withCron(t, e)
	ctx := t.Context()
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	space := e.meSpace(anu)
	otherSpace := e.meSpace(other)
	_, me := e.do("GET", "/v1/me", anu, nil)
	anuID := me["id"].(string)

	e.logPerfect(anu, dayStr(-1))
	e.enableTracker(anu)
	st, _ := e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-20), "endDate": dayStr(-16)})
	require.Equal(t, 201, st)
	chat := e.newChat(anu)
	st, _ = e.do("POST", "/v1/chat/sessions/"+chat+"/messages", anu, map[string]any{"content": "hello"})
	require.Equal(t, 202, st)

	st, _ = e.do("DELETE", "/v1/me", anu, nil)
	require.Equal(t, 202, st)

	st, b := e.purge("Bearer " + cronSecret)
	require.Equal(t, 200, st)
	require.Contains(t, b, `"erased":0`, "nothing is due during the grace period")

	e.clock.SetOffset(constants.AccountDeletionGrace + time.Minute)
	st, _ = e.purge("")
	require.Equal(t, 401, st, "the cron endpoint needs the secret")
	st, _ = e.purge("Bearer wrong")
	require.Equal(t, 401, st)

	e.mail.Messages = nil
	st, b = e.purge("Bearer " + cronSecret)
	require.Equal(t, 200, st)
	require.Contains(t, b, `"erased":1`)
	require.Contains(t, e.subjects(), "Your Bloom Health account has been deleted")

	_, err := e.repos.Users.ByEmail(ctx, "anu@example.com")
	require.ErrorIs(t, err, apperrors.ErrRepoNotFound)
	_, err = e.repos.Spaces.ByID(ctx, space)
	require.ErrorIs(t, err, apperrors.ErrRepoNotFound)
	members, err := e.repos.Memberships.MembersOfSpace(ctx, space)
	require.NoError(t, err)
	require.Empty(t, members, "the viewer's access to the space goes too")
	for typ := range mongorepo.EntityCollections {
		rows, err := e.repos.Entities.List(ctx, space, typ, mongorepo.EntityQuery{IncludeDeleted: true})
		require.NoError(t, err)
		require.Empty(t, rows, typ)
	}
	_, err = e.repos.ChatSessions.Get(ctx, anuID, chat)
	require.ErrorIs(t, err, apperrors.ErrRepoNotFound)
	msgs, err := e.repos.ChatMessages.ListForSession(ctx, chat, 0, 10)
	require.NoError(t, err)
	require.Empty(t, msgs)

	// Nobody else is touched.
	rows, err := e.repos.Entities.List(ctx, otherSpace, entitytype.Habit, mongorepo.EntityQuery{})
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	// The email is free for a new sign-up.
	e.signup("Anu", "9876500001", "anu@example.com")
}

func TestOverdueAccountIsErasedAtSignInEvenBeforeThePurge(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	st, _ := e.do("DELETE", "/v1/me", anu, nil)
	require.Equal(t, 202, st)

	e.clock.SetOffset(constants.AccountDeletionGrace + time.Minute)
	e.mail.Messages = nil
	st, _ = e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": "anu@example.com"})
	require.Equal(t, 202, st, "the same answer as for an unknown account")
	require.Empty(t, e.mail.Messages, "no code: the account no longer exists")
	_, err := e.repos.Users.ByEmail(t.Context(), "anu@example.com")
	require.ErrorIs(t, err, apperrors.ErrRepoNotFound)
}

func TestViewerDeletionKeepsTheOwnersSpace(t *testing.T) {
	e := setup(t)
	withCron(t, e)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")
	space := e.meSpace(anu)

	st, _ := e.do("DELETE", "/v1/me", aman, nil)
	require.Equal(t, 202, st)
	st, _ = e.do("GET", "/v1/habits", anu, nil)
	require.Equal(t, 200, st, "a viewer's request never locks the owner")

	e.clock.SetOffset(constants.AccountDeletionGrace + time.Minute)
	st, _ = e.purge("Bearer " + cronSecret)
	require.Equal(t, 200, st)
	_, err := e.repos.Users.ByEmail(t.Context(), "aman@example.com")
	require.ErrorIs(t, err, apperrors.ErrRepoNotFound)
	members, err := e.repos.Memberships.MembersOfSpace(t.Context(), space)
	require.NoError(t, err)
	require.Len(t, members, 1, "only the viewer's membership goes")
}

func TestPurgeEndpointIsOffWithoutASecret(t *testing.T) {
	e := setup(t)
	st, _ := e.purge("Bearer anything")
	require.Equal(t, 404, st)
}
