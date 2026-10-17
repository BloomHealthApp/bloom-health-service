package httpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bubu-bloom/service/pkg/enums/entitytype"
)

const privateNote = "ZZ-private-period-note-ZZ"

func dayStr(offset int) string { return time.Now().UTC().AddDate(0, 0, offset).Format("2006-01-02") }

var perfectDay = map[string]any{"default-wakeup": 290, "default-exercise": 100, "default-no-sugar": 1, "default-no-junk": 1,
	"default-dinner": 1000, "default-water": 2100, "default-steps": 8000, "default-sleep": 1300}

func (e *env) logPerfect(tok, date string) {
	e.t.Helper()
	for id, v := range perfectDay {
		st, body := e.do("PUT", "/v1/logs/"+id+"/"+date, tok, map[string]any{"value": v, "baseVersion": 0})
		require.Equal(e.t, 200, st, body)
	}
}

func (e *env) patchMe(tok string, body map[string]any) (int, map[string]any) {
	e.t.Helper()
	return e.do("PATCH", "/v1/me", tok, body)
}

func (e *env) setSettings(tok string, patch map[string]any) map[string]any {
	e.t.Helper()
	_, cur := e.do("GET", "/v1/settings", tok, nil)
	patch["version"] = cur["version"]
	st, body := e.do("PATCH", "/v1/settings", tok, patch)
	require.Equal(e.t, 200, st, body)
	return body
}

func (e *env) streaks(tok string) (cur, long int, days map[string]map[string]any) {
	e.t.Helper()
	st, sum := e.do("GET", "/v1/stats/summary", tok, nil)
	require.Equal(e.t, 200, st, sum)
	days = map[string]map[string]any{}
	for _, d := range sum["days"].([]any) {
		m := d.(map[string]any)
		days[m["localDate"].(string)] = m
	}
	return int(sum["currentStreak"].(float64)), int(sum["longestStreak"].(float64)), days
}

func TestGenderSignupAndTrackerDefault(t *testing.T) {
	e := setup(t)
	e.relaxLimits()

	f := e.signup2("Riya", "9876500001", "riya@example.com", map[string]any{"gender": "female"})
	u := f["user"].(map[string]any)
	require.Equal(t, "female", u["gender"])
	require.Equal(t, true, u["periodTrackerEnabled"])
	tok := f["accessToken"].(string)
	_, me := e.do("GET", "/v1/me", tok, nil)
	require.Equal(t, "female", me["gender"])
	require.Equal(t, true, me["periodTrackerEnabled"])
	st, _ := e.do("GET", "/v1/periods", tok, nil)
	require.Equal(t, 200, st)

	m := e.signup2("Mohan", "9876500002", "mohan@example.com", map[string]any{"gender": "male"})
	require.Equal(t, "male", m["user"].(map[string]any)["gender"])
	require.Equal(t, false, m["user"].(map[string]any)["periodTrackerEnabled"])
	mt := m["accessToken"].(string)
	for _, c := range []struct{ method, path string }{{"GET", "/v1/periods"}, {"POST", "/v1/periods"}, {"PATCH", "/v1/periods/x"}, {"DELETE", "/v1/periods/x"}} {
		var body any
		if c.method == "POST" || c.method == "PATCH" {
			body = map[string]any{"startDate": dayStr(-1), "version": 1}
		}
		st, resp := e.do(c.method, c.path, mt, body)
		require.Equal(t, 403, st, c.method+" "+c.path)
		require.Equal(t, "forbidden", errCode(resp))
	}

	n := e.signup2("Nita", "9876500003", "nita@example.com", nil)
	require.Nil(t, n["user"].(map[string]any)["gender"])
	require.Equal(t, false, n["user"].(map[string]any)["periodTrackerEnabled"])

	for _, v := range []any{"woman", "", "FEMALE", 1, true} {
		e.mail.Messages = nil
		st, body := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "Bad", "phone": "9876511112", "email": "bad2@example.com", "gender": v})
		require.Equal(t, 400, st, "%v", v)
		if _, isStr := v.(string); isStr {
			require.Equal(t, "validation_failed", errCode(body))
		}
		require.Empty(t, e.mail.Messages)
	}
}

func TestPatchMeGenderAndTracker(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")

	_, me := e.do("GET", "/v1/me", anu, nil)
	require.Contains(t, me, "gender")
	require.Nil(t, me["gender"])
	require.Equal(t, false, me["periodTrackerEnabled"])

	st, me := e.patchMe(anu, map[string]any{"gender": "female"})
	require.Equal(t, 200, st)
	require.Equal(t, true, me["periodTrackerEnabled"], "female defaults on")

	st, me = e.patchMe(anu, map[string]any{"periodTrackerEnabled": false})
	require.Equal(t, 200, st)
	require.Equal(t, "female", me["gender"])
	require.Equal(t, false, me["periodTrackerEnabled"], "explicit off wins over the gender default")

	st, me = e.patchMe(anu, map[string]any{"periodTrackerEnabled": nil})
	require.Equal(t, 200, st)
	require.Equal(t, true, me["periodTrackerEnabled"], "null resets to the gender default")

	st, me = e.patchMe(anu, map[string]any{"gender": "other"})
	require.Equal(t, 200, st)
	require.Equal(t, false, me["periodTrackerEnabled"])
	st, me = e.patchMe(anu, map[string]any{"periodTrackerEnabled": true})
	require.Equal(t, 200, st)
	require.Equal(t, true, me["periodTrackerEnabled"], "other can opt in")
	st, me = e.patchMe(anu, map[string]any{"gender": "prefer_not_to_say"})
	require.Equal(t, 200, st)
	require.Equal(t, true, me["periodTrackerEnabled"], "explicit setting survives a gender change")

	st, me = e.patchMe(anu, map[string]any{"gender": nil, "periodTrackerEnabled": nil})
	require.Equal(t, 200, st)
	require.Nil(t, me["gender"])
	require.Equal(t, false, me["periodTrackerEnabled"])

	for _, body := range []map[string]any{{"gender": "woman"}, {"gender": ""}, {"gender": 5}, {"gender": "Female"},
		{"periodTrackerEnabled": "yes"}, {"periodTrackerEnabled": 1}} {
		st, _ := e.patchMe(anu, body)
		require.Equal(t, 400, st, "%v", body)
	}
	_, me = e.do("GET", "/v1/me", anu, nil)
	require.Nil(t, me["gender"], "rejected patches change nothing")
}

func (e *env) enableTracker(tok string) {
	e.t.Helper()
	st, _ := e.patchMe(tok, map[string]any{"gender": "female"})
	require.Equal(e.t, 200, st)
}

func TestPeriodCRUDAndRules(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	st, _ := e.do("GET", "/v1/periods", anu, nil)
	require.Equal(t, 403, st, "tracker off by default for an account without gender")
	e.enableTracker(anu)

	st, p := e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-20), "endDate": dayStr(-16), "flow": "heavy",
		"symptoms": []any{"cramps", "cramps", "acne"}, "mood": "calm", "note": privateNote})
	require.Equal(t, 201, st, p)
	require.Equal(t, dayStr(-20), p["startDate"])
	require.Equal(t, dayStr(-16), p["endDate"])
	require.Equal(t, "heavy", p["flow"])
	require.Equal(t, []any{"cramps", "acne"}, p["symptoms"], "duplicates collapse")
	require.Equal(t, "calm", p["mood"])
	require.EqualValues(t, 1, p["version"])
	require.Nil(t, p["deletedAt"])
	require.NotEmpty(t, p["id"])
	id1 := p["id"].(string)

	// defaults on a minimal body
	st, p2 := e.do("POST", "/v1/periods", anu, map[string]any{"id": "per-2", "startDate": dayStr(-3)})
	require.Equal(t, 201, st, p2)
	require.Equal(t, "per-2", p2["id"])
	require.Nil(t, p2["endDate"], "ongoing")
	require.Equal(t, []any{}, p2["symptoms"])
	require.Nil(t, p2["mood"])
	require.Nil(t, p2["note"])
	require.NotEmpty(t, p2["flow"])

	bad := []struct {
		name string
		body map[string]any
	}{
		{"future start", map[string]any{"startDate": dayStr(1)}},
		{"bad date", map[string]any{"startDate": "2026-13-45"}},
		{"missing start", map[string]any{"flow": "light"}},
		{"end before start", map[string]any{"startDate": dayStr(-60), "endDate": dayStr(-61)}},
		{"too long", map[string]any{"startDate": dayStr(-60), "endDate": dayStr(-46)}},
		{"bad flow", map[string]any{"startDate": dayStr(-60), "flow": "gushing"}},
		{"bad mood", map[string]any{"startDate": dayStr(-60), "mood": "ecstatic"}},
		{"bad symptom", map[string]any{"startDate": dayStr(-60), "symptoms": []any{"fever"}}},
		{"too many symptoms", map[string]any{"startDate": dayStr(-60), "symptoms": []any{"cramps", "headache", "bloating", "fatigue", "mood_swings", "backache", "acne", "nausea", "cravings", "tender_breasts", "cramps2"}}},
		{"long note", map[string]any{"startDate": dayStr(-60), "note": strings.Repeat("a", 501)}},
		{"unknown field", map[string]any{"startDate": dayStr(-60), "createdBy": "x"}},
		{"overlap completed", map[string]any{"startDate": dayStr(-18), "endDate": dayStr(-15)}},
		{"overlap contained", map[string]any{"startDate": dayStr(-19), "endDate": dayStr(-17)}},
		{"overlap ongoing", map[string]any{"startDate": dayStr(-4), "endDate": dayStr(-2)}},
		{"second ongoing", map[string]any{"startDate": dayStr(-10)}},
	}
	for _, c := range bad {
		st, body := e.do("POST", "/v1/periods", anu, c.body)
		require.Equal(t, 400, st, c.name)
		require.Equal(t, "validation_failed", errCode(body), c.name)
	}
	// exactly 14 days is the maximum; adjacent (non-overlapping) ranges are fine
	st, body := e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-45), "endDate": dayStr(-32)})
	require.Equal(t, 201, st, body)
	st, _ = e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-31), "endDate": dayStr(-30)})
	require.Equal(t, 201, st)

	// PATCH needs the version; ending the ongoing period works; stale versions conflict
	st, _ = e.do("PATCH", "/v1/periods/per-2", anu, map[string]any{"endDate": dayStr(-1)})
	require.Equal(t, 400, st)
	st, up := e.do("PATCH", "/v1/periods/per-2", anu, map[string]any{"endDate": dayStr(-1), "flow": "light", "version": 1})
	require.Equal(t, 200, st, up)
	require.EqualValues(t, 2, up["version"])
	require.Equal(t, dayStr(-1), up["endDate"])
	require.Equal(t, "light", up["flow"])
	st, cf := e.do("PATCH", "/v1/periods/per-2", anu, map[string]any{"note": "x", "version": 1})
	require.Equal(t, 409, st)
	require.Equal(t, "conflict", errCode(cf))
	st, _ = e.do("PATCH", "/v1/periods/per-2", anu, map[string]any{"startDate": dayStr(-19), "version": 2})
	require.Equal(t, 400, st, "moving into another range is rejected")
	st, _ = e.do("PATCH", "/v1/periods/nope", anu, map[string]any{"note": "x", "version": 1})
	require.Equal(t, 404, st)

	st, list := e.do("GET", "/v1/periods", anu, nil)
	require.Equal(t, 200, st)
	items := list["items"].([]any)
	require.Len(t, items, 4)
	require.Equal(t, dayStr(-45), items[0].(map[string]any)["startDate"], "sorted by startDate")

	// delete is a tombstone: it frees the range and disappears from the list
	st, _ = e.do("DELETE", "/v1/periods/"+id1, anu, nil)
	require.Equal(t, 204, st)
	st, _ = e.do("DELETE", "/v1/periods/nope", anu, nil)
	require.Equal(t, 404, st)
	_, list = e.do("GET", "/v1/periods", anu, nil)
	require.Len(t, list["items"].([]any), 3)
	st, _ = e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-19), "endDate": dayStr(-17)})
	require.Equal(t, 201, st)
	st, ranged := e.do("GET", "/v1/periods?from="+dayStr(-5)+"&to="+dayStr(0), anu, nil)
	require.Equal(t, 200, st)
	require.Len(t, ranged["items"].([]any), 1)
}

func TestPeriodsPrivateFromViewers(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")
	e.enableTracker(anu)
	st, p := e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-5), "endDate": dayStr(-3), "flow": "medium",
		"symptoms": []any{"cramps"}, "mood": "sad", "note": privateNote})
	require.Equal(t, 201, st, p)
	pid := p["id"].(string)

	// the viewer is forbidden on every /periods route, even for a real id
	for _, c := range []struct{ method, path string }{{"GET", "/v1/periods"}, {"POST", "/v1/periods"}, {"PATCH", "/v1/periods/" + pid}, {"DELETE", "/v1/periods/" + pid}} {
		var body any
		if c.method == "POST" || c.method == "PATCH" {
			body = map[string]any{"startDate": dayStr(-1), "version": 1}
		}
		st, resp := e.do(c.method, c.path, aman, body)
		require.Equal(t, 403, st, c.method+" "+c.path)
		require.Equal(t, "forbidden", errCode(resp))
	}
	_, got := e.do("GET", "/v1/periods", anu, nil)
	require.Len(t, got["items"].([]any), 1, "the viewer changed nothing")

	// pull: owner sees it, viewer gets an empty periods array and no trace of the row
	st, pull := e.do("GET", "/v1/sync/pull", anu, nil)
	require.Equal(t, 200, st)
	require.Len(t, pull["changes"].(map[string]any)["periods"], 1)
	st, vpull := e.do("GET", "/v1/sync/pull?limit=1000", aman, nil)
	require.Equal(t, 200, st)
	vch := vpull["changes"].(map[string]any)
	require.Contains(t, vch, "periods")
	require.Equal(t, []any{}, vch["periods"])
	raw, _ := json.Marshal(vpull)
	require.NotContains(t, string(raw), privateNote)
	require.NotContains(t, string(raw), dayStr(-5))
	require.NotEmpty(t, vch["habits"], "other entity types still sync to viewers")

	// paging with a tiny limit still advances the viewer's cursor over the filtered rows
	cursor := ""
	for i := 0; i < 100; i++ {
		_, pg := e.do("GET", "/v1/sync/pull?limit=1&cursor="+cursor, aman, nil)
		require.Equal(t, []any{}, pg["changes"].(map[string]any)["periods"])
		cursor, _ = pg["cursor"].(string)
		if more, _ := pg["hasMore"].(bool); !more {
			break
		}
		require.NotEqual(t, 99, i, "cursor must make progress")
	}

	// push of a period by the viewer is rejected, whatever the payload
	st, resp := e.do("POST", "/v1/sync/push", aman, map[string]any{"deviceId": "d", "operations": []any{
		map[string]any{"operationId": "v-op-1", "entityType": "period", "entityId": "vp1", "op": "upsert", "baseVersion": 0,
			"payload": map[string]any{"startDate": dayStr(-9), "flow": "light"}}}})
	require.Equal(t, 403, st, resp)
	st, _ = e.do("GET", "/v1/periods", anu, nil)
	require.Equal(t, 200, st)

	// no other endpoint leaks period rows or reasons to the viewer
	for _, path := range []string{"/v1/me", "/v1/settings", "/v1/habits?includeInactive=true", "/v1/logs?from=" + dayStr(-30) + "&to=" + dayStr(0),
		"/v1/workouts", "/v1/stats/summary", "/v1/stats/summary?from=" + dayStr(-30) + "&to=" + dayStr(0)} {
		st, body := e.do("GET", path, aman, nil)
		require.Equal(t, 200, st, path)
		raw, _ := json.Marshal(body)
		s := string(raw)
		require.NotContains(t, s, privateNote, path)
		require.NotContains(t, s, "cramps", path)
		require.NotContains(t, s, "startDate", path)
		require.NotContains(t, s, "periods", path)
	}
	_, vme := e.do("GET", "/v1/me", aman, nil)
	require.Equal(t, false, vme["periodTrackerEnabled"], "the viewer's own setting; the owner's is never exposed")
	require.Nil(t, vme["gender"])
	_, vstats := e.do("GET", "/v1/stats/summary?from="+dayStr(-6)+"&to="+dayStr(-2), aman, nil)
	for _, d := range vstats["days"].([]any) {
		m := d.(map[string]any)
		require.Len(t, m, 4, "days expose only localDate, completionPercent, successful, restDay")
	}
}

func TestPeriodTrackerDisabledBlocksWrites(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	e.enableTracker(anu)
	st, p := e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-5), "endDate": dayStr(-3)})
	require.Equal(t, 201, st)

	st, me := e.patchMe(anu, map[string]any{"periodTrackerEnabled": false})
	require.Equal(t, 200, st)
	require.Equal(t, false, me["periodTrackerEnabled"])
	for _, c := range []struct{ method, path string }{{"GET", "/v1/periods"}, {"POST", "/v1/periods"}, {"PATCH", "/v1/periods/" + p["id"].(string)}, {"DELETE", "/v1/periods/" + p["id"].(string)}} {
		var body any
		if c.method == "POST" || c.method == "PATCH" {
			body = map[string]any{"startDate": dayStr(-30), "version": 1}
		}
		st, resp := e.do(c.method, c.path, anu, body)
		require.Equal(t, 403, st, c.method+" "+c.path)
		require.Equal(t, "forbidden", errCode(resp))
	}
	st, resp := e.do("POST", "/v1/sync/push", anu, map[string]any{"deviceId": "d", "operations": []any{
		map[string]any{"operationId": "off-1", "entityType": "period", "entityId": "pp", "op": "upsert", "baseVersion": 0,
			"payload": map[string]any{"startDate": dayStr(-30), "flow": "light"}}}})
	require.Equal(t, 200, st)
	r := resp["results"].([]any)[0].(map[string]any)
	require.Equal(t, "rejected", r["status"])
	require.Equal(t, "period_tracker_disabled", r["reason"])

	// re-enabling brings the data back untouched
	e.patchMe(anu, map[string]any{"periodTrackerEnabled": nil})
	_, list := e.do("GET", "/v1/periods", anu, nil)
	require.Len(t, list["items"].([]any), 1)
}

func TestPeriodSyncPushByOwner(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	e.enableTracker(anu)
	push := func(op map[string]any) map[string]any {
		st, resp := e.do("POST", "/v1/sync/push", anu, map[string]any{"deviceId": "d", "operations": []any{op}})
		require.Equal(t, 200, st, resp)
		return resp["results"].([]any)[0].(map[string]any)
	}
	r := push(map[string]any{"operationId": "o1", "entityType": "period", "entityId": "s1", "op": "upsert", "baseVersion": 0,
		"payload": map[string]any{"startDate": dayStr(-8), "endDate": dayStr(-5), "flow": "medium"}})
	require.Equal(t, "applied", r["status"])
	require.Equal(t, "period", r["entityType"])
	r = push(map[string]any{"operationId": "o2", "entityType": "period", "entityId": "s2", "op": "upsert", "baseVersion": 0,
		"payload": map[string]any{"startDate": dayStr(-6), "flow": "light"}})
	require.Equal(t, "rejected", r["status"])
	require.Contains(t, r["reason"], "overlap")
	r = push(map[string]any{"operationId": "o1", "entityType": "period", "entityId": "s1", "op": "upsert", "baseVersion": 0,
		"payload": map[string]any{"startDate": dayStr(-8), "endDate": dayStr(-5), "flow": "medium"}})
	require.Equal(t, "duplicate", r["status"])
	r = push(map[string]any{"operationId": "o3", "entityType": "period", "entityId": "s1", "op": "delete", "baseVersion": 1})
	require.Equal(t, "applied", r["status"])
	_, pull := e.do("GET", "/v1/sync/pull", anu, nil)
	periods := pull["changes"].(map[string]any)["periods"].([]any)
	require.Len(t, periods, 1)
	require.NotNil(t, periods[0].(map[string]any)["deletedAt"], "tombstones are pulled")
}

func TestSundayRestDefault(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")

	// most recent Sunday that is at least two days ago
	off := -2
	for time.Now().UTC().AddDate(0, 0, off).Weekday() != time.Sunday {
		off--
	}
	sun := dayStr(off)
	for d := off - 1; d <= -1; d++ {
		if d != off {
			e.logPerfect(anu, dayStr(d))
		}
	}
	logged := -off          // days from sun-1 .. yesterday, minus the Sunday
	sinceSunday := -1 - off // sun+1 .. yesterday

	check := func(label string, wantStreak int, wantSundayRest bool) {
		t.Helper()
		for _, tok := range []string{anu, aman} {
			cur, long, days := e.streaks(tok)
			require.Equal(t, wantStreak, cur, label)
			require.Equal(t, wantStreak, long, label)
			require.Equal(t, wantSundayRest, days[sun]["restDay"], label)
		}
	}

	// 1. sundayRest true (the seeded default)
	_, st := e.do("GET", "/v1/settings", anu, nil)
	require.Equal(t, true, st["sundayRest"])
	check("sundayRest=true", logged, true)

	// 2. field absent (legacy settings): behaves as true and GET /settings reports true
	ctx := context.Background()
	me := e.meSpace(anu)
	se, err := e.repos.Entities.Get(ctx, me, entitytype.Settings, "settings")
	require.NoError(t, err)
	next := se.Clone()
	delete(next.Data, "sundayRest")
	next.Version++
	require.NoError(t, e.repos.Entities.Save(ctx, next))
	_, st = e.do("GET", "/v1/settings", aman, nil)
	require.Equal(t, true, st["sundayRest"], "absent reads as true")
	check("sundayRest absent", logged, true)

	// 3. explicit false: the unlogged Sunday breaks the streak
	got := e.setSettings(anu, map[string]any{"sundayRest": false})
	require.Equal(t, false, got["sundayRest"])
	check("sundayRest=false", sinceSunday, false)

	// back on; a Sunday that is logged still counts as a normal success
	e.setSettings(anu, map[string]any{"sundayRest": true})
	e.logPerfect(anu, sun)
	check("sunday logged", logged+1, true)

	// validation
	_, cur := e.do("GET", "/v1/settings", anu, nil)
	stc, _ := e.do("PATCH", "/v1/settings", anu, map[string]any{"sundayRest": "no", "version": cur["version"]})
	require.Equal(t, 400, stc)
}

func (e *env) meSpace(tok string) string {
	e.t.Helper()
	_, me := e.do("GET", "/v1/me", tok, nil)
	return me["sharedSpaceId"].(string)
}

func TestPeriodDaysAreNeutralInStats(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")
	e.setSettings(anu, map[string]any{"sundayRest": false}) // isolate period behaviour from Sundays
	e.logPerfect(anu, dayStr(-5))
	e.logPerfect(anu, dayStr(-4))
	// -3 and -2 are unlogged; -1 is logged
	e.logPerfect(anu, dayStr(-1))

	// no tracker: the gap breaks the streak
	cur, long, _ := e.streaks(anu)
	require.Equal(t, 1, cur)
	require.Equal(t, 2, long)

	// a period over the gap: the streak survives, for the owner and for the viewer
	e.enableTracker(anu)
	st, p := e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-3), "endDate": dayStr(-2), "flow": "heavy", "note": privateNote})
	require.Equal(t, 201, st, p)
	for _, tok := range []string{anu, aman} {
		cur, long, days := e.streaks(tok)
		require.Equal(t, 3, cur)
		require.Equal(t, 3, long)
		require.Equal(t, true, days[dayStr(-3)]["restDay"])
		require.Equal(t, true, days[dayStr(-2)]["restDay"])
		require.Equal(t, false, days[dayStr(-4)]["restDay"])
		require.Equal(t, false, days[dayStr(-3)]["successful"])
	}

	// a logged period day counts like any other success
	e.logPerfect(anu, dayStr(-3))
	cur, _, _ = e.streaks(aman)
	require.Equal(t, 4, cur)

	// turning the tracker off stops period days being neutral (data is kept)
	e.patchMe(anu, map[string]any{"periodTrackerEnabled": false})
	cur, _, days := e.streaks(aman)
	require.Equal(t, 1, cur)
	require.Equal(t, false, days[dayStr(-2)]["restDay"])
	e.patchMe(anu, map[string]any{"periodTrackerEnabled": nil})
	cur, _, _ = e.streaks(aman)
	require.Equal(t, 4, cur)

	// a deleted period no longer neutralizes its days
	st, _ = e.do("DELETE", "/v1/periods/"+p["id"].(string), anu, nil)
	require.Equal(t, 204, st)
	cur, _, _ = e.streaks(aman)
	require.Equal(t, 1, cur)

	// an ongoing period lasts through today: today's gap never breaks anything and a yesterday gap is neutral
	st, _ = e.do("POST", "/v1/periods", anu, map[string]any{"startDate": dayStr(-2)})
	require.Equal(t, 201, st)
	cur, _, days = e.streaks(aman)
	require.Equal(t, true, days[dayStr(-2)]["restDay"])
	require.Equal(t, true, days[dayStr(0)]["restDay"])
	require.Equal(t, 4, cur, "-5,-4,-3 and -1 count, -2 is neutral")
}
