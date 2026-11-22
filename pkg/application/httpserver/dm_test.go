package httpserver_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
)

const secretText = "ZZ-private-dm-ZZ"

func (e *env) sendDM(tok, to, text string) (int, map[string]any) {
	e.t.Helper()
	return e.do("POST", "/v1/dm/"+to+"/messages", tok, map[string]any{"id": uuid.NewString(), "text": text})
}

func (e *env) pushToken(tok, token string) {
	e.t.Helper()
	st, body := e.do("PUT", "/v1/me/push-token", tok, map[string]any{"token": token, "platform": "android"})
	require.Equal(e.t, 204, st, body)
}

func TestFriendsMessageEachOther(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	stranger, _ := e.login("other-viewer@example.com")
	anuID, otherID := e.meID(anu), e.meID(other)

	st, _ := e.sendDM(other, anuID, "hi")
	require.Equal(t, 404, st, "not friends yet")
	e.befriend(anu, other)
	e.pushToken(anu, "ExponentPushToken[anu-phone]")

	// Send, and resend the same id after a lost reply: one message, 201 then 200.
	id := uuid.NewString()
	st, first := e.do("POST", "/v1/dm/"+anuID+"/messages", other, map[string]any{"id": id, "text": "  " + secretText + "  "})
	require.Equal(t, 201, st, first)
	require.Equal(t, secretText, first["text"], "trimmed")
	require.Equal(t, []string{"createdAt", "id", "senderId", "seq", "text"}, keys(first))
	st, again := e.do("POST", "/v1/dm/"+anuID+"/messages", other, map[string]any{"id": id, "text": secretText})
	require.Equal(t, 200, st, again)
	require.Equal(t, first["seq"], again["seq"])

	// The push names the sender but never carries the text.
	pushes := e.push.messages()
	require.Len(t, pushes, 1)
	require.Equal(t, "ExponentPushToken[anu-phone]", pushes[0].To)
	require.Equal(t, "Other sent you a message", pushes[0].Body)
	require.Equal(t, map[string]string{"type": "dm", "from": otherID}, pushes[0].Data)
	require.NotContains(t, fmt.Sprint(pushes), secretText)

	// Anu's inbox: one unread chat with Other's public profile only.
	st, inbox := e.do("GET", "/v1/dm/threads", anu, nil)
	require.Equal(t, 200, st, inbox)
	require.EqualValues(t, 1, inbox["unreadCount"])
	thread := list(inbox, "items")[0].(map[string]any)
	require.Equal(t, []string{"friend", "last", "unread"}, keys(thread))
	require.Equal(t, []string{"avatar", "displayName", "id"}, keys(thread["friend"].(map[string]any)))
	require.Equal(t, false, thread["last"].(map[string]any)["mine"])

	// Anu replies; pages and polling.
	for i := 0; i < 4; i++ {
		st, _ = e.sendDM(anu, otherID, fmt.Sprintf("reply %d", i))
		require.Equal(t, 201, st)
	}
	st, page := e.do("GET", "/v1/dm/"+otherID+"/messages?limit=3", anu, nil)
	require.Equal(t, 200, st, page)
	items := list(page, "items")
	require.Len(t, items, 3)
	require.Equal(t, "reply 3", items[2].(map[string]any)["text"], "oldest first, newest last")
	require.Equal(t, true, page["hasMore"])
	before := int(page["nextBefore"].(float64))
	_, older := e.do("GET", fmt.Sprintf("/v1/dm/%s/messages?before=%d", otherID, before), anu, nil)
	require.Len(t, list(older, "items"), 2)
	require.Nil(t, older["nextBefore"])
	newest := int(items[2].(map[string]any)["seq"].(float64))
	_, none := e.do("GET", fmt.Sprintf("/v1/dm/%s/messages?after=%d", otherID, newest), other, nil)
	require.Empty(t, list(none, "items"))
	st, _ = e.sendDM(other, anuID, "new one")
	require.Equal(t, 201, st)
	_, polled := e.do("GET", fmt.Sprintf("/v1/dm/%s/messages?after=%d", otherID, newest), anu, nil)
	require.Len(t, list(polled, "items"), 1)

	// Seen: Other hasn't read Anu's replies until they mark the chat read.
	_, beforeRead := e.do("GET", "/v1/dm/"+otherID+"/messages", anu, nil)
	seenBefore := beforeRead["seenAt"]
	e.clock.SetOffset(time.Minute)
	st, _ = e.do("POST", "/v1/dm/"+anuID+"/read", other, nil)
	require.Equal(t, 204, st)
	_, afterRead := e.do("GET", "/v1/dm/"+otherID+"/messages", anu, nil)
	require.NotEqual(t, seenBefore, afterRead["seenAt"])
	_, otherInbox := e.do("GET", "/v1/dm/threads", other, nil)
	require.EqualValues(t, 0, otherInbox["unreadCount"])

	// Strangers can't read, send or mark read, and get the same 404 as for an unknown id.
	for _, path := range []string{"/v1/dm/" + anuID + "/messages", "/v1/dm/nobody/messages"} {
		st, _ = e.do("GET", path, stranger, nil)
		require.Equal(t, 404, st, path)
	}
	st, _ = e.do("POST", "/v1/dm/"+anuID+"/read", stranger, nil)
	require.Equal(t, 404, st)
	st, _ = e.sendDM(anu, anuID, "me")
	require.Equal(t, 404, st, "no chat with yourself")

	// Unfriending deletes the chat for both.
	_, conns := e.do("GET", "/v1/connections", anu, nil)
	connID := list(conns, "friends")[0].(map[string]any)["id"].(string)
	st, _ = e.do("DELETE", "/v1/connections/"+connID, anu, nil)
	require.Equal(t, 204, st)
	_, gone := e.do("GET", "/v1/dm/threads", other, nil)
	require.Empty(t, list(gone, "items"))
	e.befriend(anu, other)
	_, fresh := e.do("GET", "/v1/dm/"+otherID+"/messages", anu, nil)
	require.Empty(t, list(fresh, "items"), "befriending again starts an empty chat")
}

func TestMessageValidation(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	e.befriend(anu, other)
	otherID := e.meID(other)
	for _, body := range []map[string]any{
		{"id": uuid.NewString(), "text": "   "},
		{"id": uuid.NewString(), "text": strings.Repeat("a", 1001)},
		{"id": "not-a-uuid", "text": "hi"},
		{"id": uuid.NewString(), "text": "hi", "senderId": "x"},
	} {
		st, resp := e.do("POST", "/v1/dm/"+otherID+"/messages", anu, body)
		require.Equal(t, 400, st, "%v %v", body, resp)
	}
	st, _ := e.do("GET", "/v1/dm/"+otherID+"/messages?before=2&after=1", anu, nil)
	require.Equal(t, 400, st)
	st, _ = e.do("PUT", "/v1/me/push-token", anu, map[string]any{"token": "nope", "platform": "android"})
	require.Equal(t, 400, st)
	st, _ = e.do("PUT", "/v1/me/push-token", anu, map[string]any{"token": "ExponentPushToken[x]", "platform": "web"})
	require.Equal(t, 400, st)
}

func TestUnregisteredPushTokensAreDropped(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	e.befriend(anu, other)
	anuID := e.meID(anu)
	e.pushToken(anu, "ExponentPushToken[old]")
	e.pushToken(anu, "ExponentPushToken[new]")
	e.push.unregistered["ExponentPushToken[old]"] = true
	st, _ := e.sendDM(other, anuID, "one")
	require.Equal(t, 201, st)
	toks, err := e.repos.PushTokens.ForUser(context.Background(), anuID)
	require.NoError(t, err)
	require.Len(t, toks, 1)
	require.Equal(t, "ExponentPushToken[new]", toks[0].Token)

	st, _ = e.do("DELETE", "/v1/me/push-token", anu, map[string]any{"token": "ExponentPushToken[new]"})
	require.Equal(t, 204, st)
	toks, _ = e.repos.PushTokens.ForUser(context.Background(), anuID)
	require.Empty(t, toks, "signing out removes the device")
}

func TestErasingAnAccountRemovesItsChatsAndTokens(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	e.befriend(anu, other)
	anuID, otherID := e.meID(anu), e.meID(other)
	e.pushToken(anu, "ExponentPushToken[anu]")
	st, msg := e.sendDM(other, anuID, secretText)
	require.Equal(t, 201, st)

	ctx := context.Background()
	u, err := e.repos.Users.ByID(ctx, anuID)
	require.NoError(t, err)
	require.NoError(t, e.repos.Accounts.EraseUser(ctx, u))

	_, err = e.repos.DMs.ByID(ctx, msg["id"].(string))
	require.Error(t, err, "the friend's messages to them are gone too")
	_, err = e.repos.DMs.Thread(ctx, dao.PairKey(anuID, otherID))
	require.Error(t, err)
	toks, _ := e.repos.PushTokens.ForUser(ctx, anuID)
	require.Empty(t, toks)
}
