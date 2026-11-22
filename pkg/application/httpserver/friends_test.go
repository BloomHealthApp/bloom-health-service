package httpserver_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
)

// linkCode returns the caller's profile link code.
func (e *env) linkCode(token string) string {
	e.t.Helper()
	st, body := e.do("GET", "/v1/me/profile-link", token, nil)
	require.Equal(e.t, 200, st, body)
	code := body["code"].(string)
	require.Len(e.t, code, constants.ProfileLinkCodeLen)
	require.True(e.t, strings.HasSuffix(body["url"].(string), "/u/"+code))
	return code
}

func (e *env) meID(token string) string {
	e.t.Helper()
	_, me := e.do("GET", "/v1/me", token, nil)
	return me["id"].(string)
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func list(body map[string]any, name string) []any {
	v, _ := body[name].([]any)
	return v
}

func TestFriendsFlowThroughAProfileLink(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	stranger, _ := e.login("other-viewer@example.com")
	anuID := e.meID(anu)

	code := e.linkCode(anu)
	require.Equal(t, code, e.linkCode(anu), "the code is stable until reset")

	// Before they are friends, the link shows only name and avatar: no stats, no private fields.
	st, view := e.do("GET", "/v1/profiles/by-link/"+code, other, nil)
	require.Equal(t, 200, st, view)
	require.Equal(t, "none", view["relation"])
	require.Equal(t, []string{"avatar", "displayName", "id"}, keys(view["profile"].(map[string]any)))

	st, sent := e.do("POST", "/v1/connections", other, map[string]any{"code": code})
	require.Equal(t, 201, st, sent)
	require.Equal(t, "outgoing", sent["relation"])
	st, _ = e.do("POST", "/v1/connections", other, map[string]any{"code": code})
	require.Equal(t, 409, st, "one request per pair")

	_, anuList := e.do("GET", "/v1/connections", anu, nil)
	require.Len(t, list(anuList, "incoming"), 1)
	req := list(anuList, "incoming")[0].(map[string]any)
	require.Equal(t, []string{"avatar", "displayName", "id"}, keys(req["profile"].(map[string]any)))

	st, _ = e.do("POST", "/v1/connections/"+req["id"].(string)+"/accept", stranger, nil)
	require.Equal(t, 404, st, "only the addressee can accept")
	st, acc := e.do("POST", "/v1/connections/"+req["id"].(string)+"/accept", anu, nil)
	require.Equal(t, 200, st, acc)

	_, otherList := e.do("GET", "/v1/connections", other, nil)
	require.Len(t, list(otherList, "friends"), 1, "friendship is mutual")
	st, prof := e.do("GET", "/v1/profiles/"+anuID, other, nil)
	require.Equal(t, 200, st, prof)
	require.Equal(t, []string{"avatar", "displayName", "id", "stats"}, keys(prof))
	require.Equal(t, []string{"currentStreak", "habits", "longestStreak"}, keys(prof["stats"].(map[string]any)))
	require.Greater(t, prof["stats"].(map[string]any)["habits"].(float64), float64(0))

	st, _ = e.do("GET", "/v1/profiles/"+anuID, stranger, nil)
	require.Equal(t, 404, st, "strangers can't see a profile by id")

	st, _ = e.do("DELETE", "/v1/connections/"+req["id"].(string), other, nil)
	require.Equal(t, 204, st)
	st, _ = e.do("GET", "/v1/profiles/"+anuID, other, nil)
	require.Equal(t, 404, st, "unfriended")
}

func TestFriendRequestsBothWaysAndDeclineCooldown(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	anuCode, otherCode := e.linkCode(anu), e.linkCode(other)

	st, _ := e.do("POST", "/v1/connections", other, map[string]any{"code": anuCode})
	require.Equal(t, 201, st)
	st, back := e.do("POST", "/v1/connections", anu, map[string]any{"code": otherCode})
	require.Equal(t, 201, st, back)
	require.Equal(t, "friends", back["relation"], "asking back accepts")
	require.Contains(t, keys(back["profile"].(map[string]any)), "stats")

	st, _ = e.do("DELETE", "/v1/connections/"+back["connectionId"].(string), anu, nil)
	require.Equal(t, 204, st)

	st, _ = e.do("POST", "/v1/connections", other, map[string]any{"code": anuCode})
	require.Equal(t, 201, st)
	_, l := e.do("GET", "/v1/connections", anu, nil)
	id := list(l, "incoming")[0].(map[string]any)["id"].(string)
	st, _ = e.do("DELETE", "/v1/connections/"+id, anu, nil)
	require.Equal(t, 404, st, "the addressee declines instead of deleting")
	st, _ = e.do("POST", "/v1/connections/"+id+"/decline", anu, nil)
	require.Equal(t, 204, st)

	st, view := e.do("GET", "/v1/profiles/by-link/"+anuCode, other, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "none", view["relation"], "a decline isn't shown as such")
	st, _ = e.do("POST", "/v1/connections", other, map[string]any{"code": anuCode})
	require.Equal(t, 429, st, "cooldown after a decline")

	e.clock.SetOffset(constants.DeclinedRequestCooldown + time.Hour)
	other, _ = e.login("other-owner@example.com") // the old access token expired with the jump
	st, _ = e.do("POST", "/v1/connections", other, map[string]any{"code": anuCode})
	require.Equal(t, 201, st, "can ask again after the cooldown")
}

func TestProfileLinksAnswerTheSameWhenUnknownResetOrMalformed(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	old := e.linkCode(anu)

	st, reset := e.do("POST", "/v1/me/profile-link/reset", anu, nil)
	require.Equal(t, 200, st)
	require.NotEqual(t, old, reset["code"])

	var bodies []map[string]any
	for _, code := range []string{old, "AAAAAAAAAAAA", "short", "has%20space!!"} {
		st, body := e.do("GET", "/v1/profiles/by-link/"+code, other, nil)
		require.Equal(t, 404, st, code)
		bodies = append(bodies, body)
	}
	for _, b := range bodies[1:] {
		require.Equal(t, bodies[0], b, "no way to tell a reset link from an unknown one")
	}
	st, own := e.do("GET", "/v1/profiles/by-link/"+reset["code"].(string), anu, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "self", own["relation"])
	st, _ = e.do("POST", "/v1/connections", anu, map[string]any{"code": reset["code"]})
	require.Equal(t, 400, st, "can't befriend yourself")
}

func TestErasingAnAccountRemovesItsConnections(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	st, _ := e.do("POST", "/v1/connections", other, map[string]any{"code": e.linkCode(anu)})
	require.Equal(t, 201, st)

	ctx := context.Background()
	u, err := e.repos.Users.ByEmail(ctx, "other-owner@example.com")
	require.NoError(t, err)
	require.NoError(t, e.repos.Accounts.EraseUser(ctx, u))
	left, err := e.repos.Connections.ListForUser(ctx, e.meID(anu), 10)
	require.NoError(t, err)
	require.Empty(t, left)
}

func (e *env) get(path string) (int, string, http.Header) {
	e.t.Helper()
	resp, err := http.Get(e.srv.URL + path)
	require.NoError(e.t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

func TestPublicProfilePageShowsOnlyAFirstName(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	code := e.linkCode(anu)

	st, page, h := e.get("/u/" + code)
	require.Equal(t, 200, st)
	require.Contains(t, page, "Anu")
	require.Contains(t, page, "bloomhealth://u/"+code)
	require.NotContains(t, page, "anu@example.com")
	require.NotContains(t, page, "streak")
	require.Contains(t, h.Get("X-Robots-Tag"), "noindex")

	st, unknown, _ := e.get("/u/AAAAAAAAAAAA")
	require.Equal(t, 404, st)
	require.Contains(t, unknown, "This link isn")

	st, links, _ := e.get("/.well-known/assetlinks.json")
	require.Equal(t, 200, st)
	require.JSONEq(t, "[]", links, "no fingerprints configured")
}

// befriend makes a and b friends through a's link.
func (e *env) befriend(a, b string) {
	e.t.Helper()
	st, sent := e.do("POST", "/v1/connections", b, map[string]any{"code": e.linkCode(a)})
	require.Equal(e.t, 201, st, sent)
	st, acc := e.do("POST", "/v1/connections/"+sent["connectionId"].(string)+"/accept", a, nil)
	require.Equal(e.t, 200, st, acc)
}

func TestFriendActivityShowsProgressButNotRestOrPeriodDays(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	stranger, _ := e.login("other-viewer@example.com")
	anuID := e.meID(anu)

	e.enableTracker(anu)
	e.logPerfect(anu, dayStr(-2))
	e.logPerfect(anu, today())
	// A period over days with nothing logged must look exactly like days that were simply missed.
	st, p := e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-6), "endDate": dayStr(-4), "note": privateNote})
	require.Equal(t, 201, st, p)

	st, _ = e.do("GET", "/v1/profiles/"+anuID+"/activity", stranger, nil)
	require.Equal(t, 404, st, "strangers can't see activity")

	e.befriend(anu, other)
	st, act := e.do("GET", "/v1/profiles/"+anuID+"/activity", other, nil)
	require.Equal(t, 200, st, act)
	require.Equal(t, []string{"days", "todayHabits"}, keys(act))

	levels := map[string]float64{}
	for _, d := range list(act, "days") {
		m := d.(map[string]any)
		require.Equal(t, []string{"date", "level"}, keys(m))
		levels[m["date"].(string)] = m["level"].(float64)
	}
	require.Equal(t, float64(4), levels[today()])
	require.Equal(t, float64(4), levels[dayStr(-2)])
	for _, off := range []int{-6, -5, -4, -3} {
		_, present := levels[dayStr(off)]
		require.False(t, present, "quiet days are left out, whatever the reason")
	}

	habits := list(act, "todayHabits")
	require.NotEmpty(t, habits)
	for _, h := range habits {
		m := h.(map[string]any)
		require.Equal(t, []string{"done", "icon", "title"}, keys(m))
		require.Equal(t, true, m["done"])
	}
	require.NotContains(t, fmt.Sprint(act), privateNote)
}
