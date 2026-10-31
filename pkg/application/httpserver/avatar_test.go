package httpserver_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchMeAvatar(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")

	st, me := e.do("GET", "/v1/me", anu, nil)
	require.Equal(t, 200, st)
	require.Contains(t, me, "avatar")
	require.Nil(t, me["avatar"], "legacy users have a null avatar")
	require.Contains(t, me, "ownerAvatar")
	require.Nil(t, me["ownerAvatar"])

	st, me = e.do("PATCH", "/v1/me", anu, map[string]any{"avatar": "bubu-03"})
	require.Equal(t, 200, st)
	require.Equal(t, "bubu-03", me["avatar"])
	require.Equal(t, "bubu-03", me["ownerAvatar"], "an owner's ownerAvatar is their own avatar")
	_, me = e.do("GET", "/v1/me", anu, nil)
	require.Equal(t, "bubu-03", me["avatar"])

	st, me = e.do("PATCH", "/v1/me", anu, map[string]any{"avatar": nil})
	require.Equal(t, 200, st)
	require.Nil(t, me["avatar"])
	require.Nil(t, me["ownerAvatar"])

	bad := []any{"evil", "bubu-00", "bubu-16", "BUBU-01", " bubu-01", "bubu-01 ", "", "http://x/y.png", strings.Repeat("a", 500), 3, true}
	for _, v := range bad {
		st, body := e.do("PATCH", "/v1/me", anu, map[string]any{"avatar": v})
		require.Equal(t, 400, st, "%v", v)
		if _, isStr := v.(string); isStr {
			require.Equal(t, "validation_failed", errCode(body), "%v", v)
		}
	}
	_, me = e.do("GET", "/v1/me", anu, nil)
	require.Nil(t, me["avatar"], "rejected patches change nothing")
}

func TestViewerSeesOwnerAvatar(t *testing.T) {
	e := setup(t)
	owner, _ := e.login("anu@example.com")
	viewer, _ := e.login("aman@example.com")

	_, me := e.do("GET", "/v1/me", viewer, nil)
	require.Equal(t, "viewer", me["role"])
	require.Contains(t, me, "ownerAvatar")
	require.Nil(t, me["ownerAvatar"])

	st, _ := e.do("PATCH", "/v1/me", owner, map[string]any{"avatar": "bubu-05"})
	require.Equal(t, 200, st)
	_, me = e.do("GET", "/v1/me", viewer, nil)
	require.Equal(t, "bubu-05", me["ownerAvatar"])
	require.Nil(t, me["avatar"], "the viewer's own avatar is unaffected")

	// the viewer may set their own avatar without changing ownerAvatar
	st, me = e.do("PATCH", "/v1/me", viewer, map[string]any{"avatar": "bubu-09"})
	require.Equal(t, 200, st)
	require.Equal(t, "bubu-09", me["avatar"])
	require.Equal(t, "bubu-05", me["ownerAvatar"])

	// and it follows later owner changes
	e.do("PATCH", "/v1/me", owner, map[string]any{"avatar": "bubu-01"})
	_, me = e.do("GET", "/v1/me", viewer, nil)
	require.Equal(t, "bubu-01", me["ownerAvatar"])
}

func TestSignUpWithOptionalAvatar(t *testing.T) {
	e := setup(t)
	e.relaxLimits()

	// with avatar: applied at verify, visible in verify response, GET /me and refresh
	body := e.signup2("Riya", "9876543210", "riya@example.com", map[string]any{"avatar": "bubu-07"})
	user := body["user"].(map[string]any)
	require.Equal(t, "bubu-07", user["avatar"])
	require.Equal(t, "bubu-07", user["ownerAvatar"])
	_, me := e.do("GET", "/v1/me", body["accessToken"].(string), nil)
	require.Equal(t, "bubu-07", me["avatar"])
	st, rf := e.do("POST", "/v1/auth/refresh", "", map[string]any{"refreshToken": body["refreshToken"]})
	require.Equal(t, 200, st)
	require.Equal(t, "bubu-07", rf["user"].(map[string]any)["ownerAvatar"])

	// absent and explicit null: account starts without an avatar
	for i, extra := range []map[string]any{{}, {"avatar": nil}} {
		phone := []string{"9876500001", "9876500002"}[i]
		email := []string{"a1@example.com", "a2@example.com"}[i]
		u := e.signup2("Noa", phone, email, extra)["user"].(map[string]any)
		require.Contains(t, u, "avatar")
		require.Nil(t, u["avatar"])
		require.Nil(t, u["ownerAvatar"])
	}
}

func TestSignUpInvalidAvatarIs400AndSendsNoMail(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	for _, v := range []any{"evil", "bubu-16", "", "BUBU-01", 7, strings.Repeat("a", 300)} {
		e.mail.Messages = nil
		st, body := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "Bad", "phone": "9876511111", "email": "bad@example.com", "avatar": v})
		require.Equal(t, 400, st, "%v", v)
		if _, isStr := v.(string); isStr {
			require.Equal(t, "validation_failed", errCode(body))
		}
		require.Empty(t, e.mail.Messages, "no mail for an invalid request")
	}
	// strict unknown-field rejection still holds
	st, _ := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "X", "phone": "9876511111", "email": "bad@example.com", "role": "owner"})
	require.Equal(t, 400, st)
	st, _ = e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "X", "phone": "9876511111", "email": "bad@example.com", "avatarUrl": "x"})
	require.Equal(t, 400, st)
}

// signup2 is signup with extra signup/request fields (e.g. avatar).
func (e *env) signup2(name, phone, email string, extra map[string]any) map[string]any {
	e.t.Helper()
	e.mail.Messages = nil
	req := map[string]any{"name": name, "phone": phone, "email": email}
	for k, v := range extra {
		req[k] = v
	}
	st, resp := e.do("POST", "/v1/auth/signup/request", "", req)
	require.Equal(e.t, 202, st, resp)
	require.Len(e.t, e.mail.Messages, 1)
	code := codeRe.FindString(e.mail.Messages[0].Body)
	st, body := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": resp["challengeId"], "code": code})
	require.Equal(e.t, 200, st, body)
	return body
}
