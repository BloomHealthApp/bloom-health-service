package httpserver_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/chatstatus"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo/memory"
)

// settle answers a pending assistant message the way bloom-health-task-worker does.
func (e *env) settle(sessionID, messageID, status, content string) {
	e.t.Helper()
	require.True(e.t, memory.SettleForTest(e.repos.ChatMessages, messageID, func(m *dao.ChatMessage) {
		m.Status, m.Content = status, content
		if status == chatstatus.Failed {
			m.ErrorCode = "provider_error"
		}
	}))
	require.NoError(e.t, e.repos.ChatSessions.EndTurn(e.t.Context(), sessionID, messageID))
}

func (e *env) newChat(token string) string {
	e.t.Helper()
	st, body := e.do("POST", "/v1/chat/sessions", token, map[string]any{})
	require.Equal(e.t, 201, st, body)
	return body["id"].(string)
}

func TestChatModels(t *testing.T) {
	e := setup(t)
	tok, _ := e.login("anu@example.com")
	st, body := e.do("GET", "/v1/chat/models", tok, nil)
	require.Equal(t, 200, st)
	items := body["items"].([]any)
	require.Len(t, items, 1)
	m := items[0].(map[string]any)
	require.Equal(t, "groq-llama-3.3-70b", m["id"])
	require.Equal(t, true, m["default"])
	require.NotContains(t, m, "providerModelId")
}

func TestChatSendAndPoll(t *testing.T) {
	e := setup(t)
	tok, _ := e.login("anu@example.com")
	sid := e.newChat(tok)

	st, body := e.do("GET", "/v1/chat/sessions/"+sid, tok, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "New chat", body["title"])
	require.Equal(t, "groq-llama-3.3-70b", body["modelId"])
	require.Nil(t, body["pendingMessageId"])
	require.Equal(t, []any{}, body["contextScopes"])

	st, body = e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": "  Tips for a 10-minute morning stretch?  "})
	require.Equal(t, 202, st, body)
	user := body["userMessage"].(map[string]any)
	asst := body["assistantMessage"].(map[string]any)
	require.Equal(t, "Tips for a 10-minute morning stretch?", user["content"])
	require.Equal(t, "succeeded", user["status"])
	require.EqualValues(t, 0, user["idx"])
	require.Equal(t, "pending", asst["status"])
	require.Equal(t, "", asst["content"])
	require.EqualValues(t, 1, asst["idx"])
	require.Nil(t, asst["errorCode"])
	asstID := asst["id"].(string)

	st, body = e.do("GET", "/v1/chat/sessions/"+sid, tok, nil)
	require.Equal(t, 200, st)
	require.Equal(t, asstID, body["pendingMessageId"])
	require.Equal(t, "Tips for a 10-minute morning stretch?", body["title"])

	st, body = e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": "again"})
	require.Equal(t, 409, st)
	require.Equal(t, "conflict", errCode(body))
	require.Equal(t, "turn_in_flight", body["error"].(map[string]any)["details"].(map[string]any)["reason"])

	e.settle(sid, asstID, chatstatus.Succeeded, "Try neck rolls.")
	st, body = e.do("GET", "/v1/chat/sessions/"+sid+"/messages", tok, nil)
	require.Equal(t, 200, st)
	items := body["items"].([]any)
	require.Len(t, items, 2)
	require.Equal(t, "user", items[0].(map[string]any)["role"])
	require.Equal(t, "Try neck rolls.", items[1].(map[string]any)["content"])
	require.Equal(t, "succeeded", items[1].(map[string]any)["status"])
	require.Equal(t, false, body["hasMore"])
	require.Nil(t, body["nextBefore"])

	st, _ = e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": "thanks"})
	require.Equal(t, 202, st)
}

func TestChatRunningIsPendingAndFailedHasErrorCode(t *testing.T) {
	e := setup(t)
	tok, _ := e.login("anu@example.com")
	sid := e.newChat(tok)
	_, body := e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": "hi"})
	asstID := body["assistantMessage"].(map[string]any)["id"].(string)

	memory.SettleForTest(e.repos.ChatMessages, asstID, func(m *dao.ChatMessage) { m.Status = chatstatus.Running })
	_, body = e.do("GET", "/v1/chat/sessions/"+sid+"/messages", tok, nil)
	require.Equal(t, "pending", body["items"].([]any)[1].(map[string]any)["status"])

	e.settle(sid, asstID, chatstatus.Failed, "")
	_, body = e.do("GET", "/v1/chat/sessions/"+sid+"/messages", tok, nil)
	last := body["items"].([]any)[1].(map[string]any)
	require.Equal(t, "failed", last["status"])
	require.Equal(t, "provider_error", last["errorCode"])
}

func TestChatPaging(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	tok, _ := e.login("anu@example.com")
	sid := e.newChat(tok)
	for i := 0; i < 3; i++ {
		_, body := e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": fmt.Sprintf("q%d", i)})
		e.settle(sid, body["assistantMessage"].(map[string]any)["id"].(string), chatstatus.Succeeded, fmt.Sprintf("a%d", i))
	}
	st, body := e.do("GET", "/v1/chat/sessions/"+sid+"/messages?limit=4", tok, nil)
	require.Equal(t, 200, st)
	items := body["items"].([]any)
	require.Len(t, items, 4)
	require.EqualValues(t, 2, items[0].(map[string]any)["idx"])
	require.EqualValues(t, 5, items[3].(map[string]any)["idx"])
	require.Equal(t, true, body["hasMore"])
	require.EqualValues(t, 2, body["nextBefore"])

	_, body = e.do("GET", "/v1/chat/sessions/"+sid+"/messages?limit=4&before=2", tok, nil)
	items = body["items"].([]any)
	require.Len(t, items, 2)
	require.Equal(t, "q0", items[0].(map[string]any)["content"])
	require.Equal(t, false, body["hasMore"])

	for _, q := range []string{"limit=0", "limit=51", "limit=x", "before=0", "before=-1"} {
		st, _ := e.do("GET", "/v1/chat/sessions/"+sid+"/messages?"+q, tok, nil)
		require.Equal(t, 400, st, q)
	}
}

func TestChatValidation(t *testing.T) {
	e := setup(t)
	tok, _ := e.login("anu@example.com")
	sid := e.newChat(tok)
	cases := []struct {
		path string
		body map[string]any
	}{
		{"/v1/chat/sessions", map[string]any{"title": "   "}},
		{"/v1/chat/sessions", map[string]any{"title": strings.Repeat("a", 81)}},
		{"/v1/chat/sessions", map[string]any{"modelId": "gpt-unknown"}},
		{"/v1/chat/sessions", map[string]any{"contextScopes": []string{"habitStreaks"}}},
		{"/v1/chat/sessions", map[string]any{"userId": "someone-else"}},
		{"/v1/chat/sessions/" + sid + "/messages", map[string]any{"content": "   "}},
		{"/v1/chat/sessions/" + sid + "/messages", map[string]any{"content": strings.Repeat("a", 4001)}},
		{"/v1/chat/sessions/" + sid + "/messages", map[string]any{"content": "hi", "modelId": "gpt-unknown"}},
		{"/v1/chat/sessions/" + sid + "/messages", map[string]any{"content": "hi", "role": "assistant"}},
	}
	for _, c := range cases {
		st, body := e.do("POST", c.path, tok, c.body)
		require.Equal(t, 400, st, c.body)
		require.Equal(t, "validation_failed", errCode(body))
	}
	st, body := e.do("POST", "/v1/chat/sessions", tok, map[string]any{"title": "  Sleep  ", "modelId": "groq-llama-3.3-70b"})
	require.Equal(t, 201, st)
	require.Equal(t, "Sleep", body["title"])
	st, _ = e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": strings.Repeat("é", 4000)})
	require.Equal(t, 202, st)
}

func TestChatSessionsArePrivateToTheUser(t *testing.T) {
	e := setup(t)
	owner, _ := e.login("anu@example.com")
	viewer, _ := e.login("aman@example.com")
	sid := e.newChat(owner)

	// The viewer shares the owner's space but not their chats; viewers may chat themselves.
	for _, r := range []struct{ method, path string }{
		{"GET", "/v1/chat/sessions/" + sid}, {"GET", "/v1/chat/sessions/" + sid + "/messages"},
		{"POST", "/v1/chat/sessions/" + sid + "/messages"}, {"DELETE", "/v1/chat/sessions/" + sid},
	} {
		var body any
		if r.method == "POST" {
			body = map[string]any{"content": "hi"}
		}
		st, _ := e.do(r.method, r.path, viewer, body)
		require.Equal(t, 404, st, r)
	}
	_, list := e.do("GET", "/v1/chat/sessions", viewer, nil)
	require.Empty(t, list["items"])
	vsid := e.newChat(viewer)
	st, _ := e.do("POST", "/v1/chat/sessions/"+vsid+"/messages", viewer, map[string]any{"content": "hello"})
	require.Equal(t, 202, st)

	_, list = e.do("GET", "/v1/chat/sessions", owner, nil)
	require.Len(t, list["items"], 1)

	st, _ = e.do("GET", "/v1/chat/sessions", "", nil)
	require.Equal(t, 401, st)
}

func TestChatDeleteSession(t *testing.T) {
	e := setup(t)
	tok, _ := e.login("anu@example.com")
	sid := e.newChat(tok)
	st, _ := e.do("DELETE", "/v1/chat/sessions/"+sid, tok, nil)
	require.Equal(t, 204, st)
	st, _ = e.do("GET", "/v1/chat/sessions/"+sid, tok, nil)
	require.Equal(t, 404, st)
	st, _ = e.do("DELETE", "/v1/chat/sessions/"+sid, tok, nil)
	require.Equal(t, 404, st)
	st, _ = e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": "hi"})
	require.Equal(t, 404, st)
	_, list := e.do("GET", "/v1/chat/sessions", tok, nil)
	require.Empty(t, list["items"])
}

func TestChatDailyCap(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	tok, _ := e.login("anu@example.com")
	sid := e.newChat(tok)
	for i := 0; i < 100; i++ {
		st, body := e.do("POST", "/v1/chat/sessions/"+sid+"/messages", tok, map[string]any{"content": "q"})
		require.Equal(t, 202, st, i)
		e.settle(sid, body["assistantMessage"].(map[string]any)["id"].(string), chatstatus.Succeeded, "a")
	}
	resp, raw := raw(t, e, "POST", "/v1/chat/sessions/"+sid+"/messages", tok, `{"content":"one more"}`)
	require.Equal(t, 429, resp.StatusCode, raw)
	require.Contains(t, raw, `"rate_limited"`)
	require.NotEmpty(t, resp.Header.Get("Retry-After"))
}
