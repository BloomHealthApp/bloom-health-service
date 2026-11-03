package httpserver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

func TestDeleteMeErasesOwnerAndSpace(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	anu, refresh := e.login("anu@example.com")
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
	otherChat := e.newChat(other)

	st, _ = e.do("DELETE", "/v1/me", anu, nil)
	require.Equal(t, 204, st)

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

	st, _ = e.do("GET", "/v1/me", anu, nil)
	require.Equal(t, 401, st, "the access token stops working at once")
	st, _ = e.do("POST", "/v1/auth/refresh", "", map[string]any{"refreshToken": refresh})
	require.Equal(t, 401, st)

	// Nobody else is touched.
	rows, err := e.repos.Entities.List(ctx, otherSpace, entitytype.Habit, mongorepo.EntityQuery{})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	st, _ = e.do("GET", "/v1/chat/sessions/"+otherChat, other, nil)
	require.Equal(t, 200, st)

	// The email is free for a new sign-up.
	e.signup("Anu", "9876500001", "anu@example.com")
}

func TestDeleteMeAsViewerKeepsTheOwnersSpace(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")
	space := e.meSpace(anu)

	st, _ := e.do("DELETE", "/v1/me", aman, nil)
	require.Equal(t, 204, st)

	_, err := e.repos.Users.ByEmail(t.Context(), "aman@example.com")
	require.ErrorIs(t, err, apperrors.ErrRepoNotFound)
	members, err := e.repos.Memberships.MembersOfSpace(t.Context(), space)
	require.NoError(t, err)
	require.Len(t, members, 1, "only the viewer's membership goes")
	st, _ = e.do("GET", "/v1/habits", anu, nil)
	require.Equal(t, 200, st)
}
