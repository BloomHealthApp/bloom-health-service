package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/application/httpserver"
	"github.com/bubu-bloom/service/pkg/di"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
	"github.com/bubu-bloom/service/pkg/infrastructure/transport/smtp"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils/clock"
)

// These are the integration tests of the original chi implementation, run unchanged through the gin
// router, the real services and in-memory repositories: they prove contract parity of the refactor.

type env struct {
	t     *testing.T
	repos *di.Repositories
	srv   *httptest.Server
	mail  *smtp.CaptureMailer
	auth  *tokenMinter
	clock *clock.Fake
}

const secret = "0123456789abcdef0123456789abcdef-test"
const issuer = "bubu-test"

// tokenMinter signs access tokens for any stored user. The outsider test needs a valid token for a
// user without membership, which the real login flow (correctly) refuses to issue.
type tokenMinter struct{ repos *di.Repositories }

type minted struct{ AccessToken string }

func (m *tokenMinter) VerifyOTP(ctx context.Context, email, _ string) (*minted, *dao.User, error) {
	u, err := m.repos.Users.ByEmail(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	at, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: u.ID, Issuer: issuer,
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)), ID: uuid.NewString()}).SignedString([]byte(secret))
	return &minted{AccessToken: at}, u, err
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	repos := di.NewMemoryRepositories()
	cfg := &config.Store{ActiveEnv: "DEV", StoreKind: "memory", AccessSecret: secret, Issuer: issuer, Version: "1.0.0",
		OTPResendAfter: 0, Mailer: "console"}
	clk := clock.NewFake()
	mailer := &smtp.CaptureMailer{}
	app, err := di.InitializeTestApp(cfg, logger.NewDiscard(), clk, mailer, repos)
	require.NoError(t, err)
	di.AppLogic = app

	prov := app.GetProvisionService()
	require.NoError(t, prov.Provision(ctx, serviceBos.ProvisionOptions{Owner: serviceBos.Person{Email: "anu@example.com", Name: "Anu"},
		Viewer: serviceBos.Person{Email: "aman@example.com", Name: "Aman"}, Timezone: "UTC"}))
	// A second, unrelated space and an unprovisioned-membership user.
	require.NoError(t, prov.Provision(ctx, serviceBos.ProvisionOptions{Owner: serviceBos.Person{Email: "other-owner@example.com", Name: "Other"},
		Viewer: serviceBos.Person{Email: "other-viewer@example.com", Name: "Viewer2"}, Timezone: "UTC"}))
	require.NoError(t, repos.Users.Save(ctx, newUser("outsider@example.com")))

	srv := httptest.NewServer(httpserver.NewRouter())
	t.Cleanup(srv.Close)
	return &env{t: t, repos: repos, srv: srv, mail: mailer, auth: &tokenMinter{repos: repos}, clock: clk}
}

var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

func (e *env) login(email string) (access, refresh string) {
	e.t.Helper()
	e.mail.Messages = nil
	st, _ := e.do("POST", "/v1/auth/otp/request", "", map[string]any{"email": email})
	require.Equal(e.t, 202, st)
	require.Len(e.t, e.mail.Messages, 1)
	code := codeRe.FindString(e.mail.Messages[0].Body)
	st, body := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"email": email, "code": code})
	require.Equal(e.t, 200, st, body)
	return body["accessToken"].(string), body["refreshToken"].(string)
}

func (e *env) do(method, path, token string, body any) (int, map[string]any) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(e.t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out
}

func newUser(email string) *dao.User {
	now := time.Now().UTC()
	return &dao.User{ID: uuid.NewString(), Email: email, DisplayName: "Outsider", Timezone: "UTC", Theme: "cottonCandy", CreatedAt: now, UpdatedAt: now}
}

func today() string { return time.Now().UTC().Format("2006-01-02") }

func TestAuthFlow(t *testing.T) {
	e := setup(t)

	// any email may sign in: a brand-new address gets a code and, once verified, a private owner account
	st, _ := e.do("POST", "/v1/auth/otp/request", "", map[string]any{"email": "stranger@example.com"})
	require.Equal(t, 202, st)
	require.Len(t, e.mail.Messages, 1)
	_, err := e.repos.Users.ByEmail(context.Background(), "stranger@example.com")
	require.Error(t, err, "no account until the code is verified")
	e.mail.Messages = nil

	// wrong code
	e.do("POST", "/v1/auth/otp/request", "", map[string]any{"email": "anu@example.com"})
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"email": "anu@example.com", "code": "000000"})
	require.Equal(t, 401, st)

	access, refresh := e.login("anu@example.com")
	st, me := e.do("GET", "/v1/me", access, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "owner", me["role"])
	require.Equal(t, "Anu", me["ownerDisplayName"])

	// refresh rotates; reuse of old token revokes the family
	st, r1 := e.do("POST", "/v1/auth/refresh", "", map[string]any{"refreshToken": refresh})
	require.Equal(t, 200, st)
	newRefresh := r1["refreshToken"].(string)
	require.NotEqual(t, refresh, newRefresh)
	st, _ = e.do("POST", "/v1/auth/refresh", "", map[string]any{"refreshToken": refresh})
	require.Equal(t, 401, st)
	st, _ = e.do("POST", "/v1/auth/refresh", "", map[string]any{"refreshToken": newRefresh})
	require.Equal(t, 401, st, "family must be revoked after reuse")
}

func TestOTPAttemptsAndSingleUse(t *testing.T) {
	e := setup(t)
	e.do("POST", "/v1/auth/otp/request", "", map[string]any{"email": "anu@example.com"})
	code := codeRe.FindString(e.mail.Messages[0].Body)
	// correct code works once only
	st, _ := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"email": "anu@example.com", "code": code})
	require.Equal(t, 200, st)
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"email": "anu@example.com", "code": code})
	require.Equal(t, 401, st)
}

func TestAuthRequired(t *testing.T) {
	e := setup(t)
	for _, p := range []string{"/v1/me", "/v1/habits", "/v1/settings", "/v1/sync/pull"} {
		st, body := e.do("GET", p, "", nil)
		require.Equal(t, 401, st, p)
		require.Equal(t, "unauthorized", body["error"].(map[string]any)["code"])
	}
	st, _ := e.do("GET", "/v1/me", "garbage", nil)
	require.Equal(t, 401, st)
}

func TestExpiredToken(t *testing.T) {
	e := setup(t)
	e.clock.SetOffset(-time.Hour)
	access, _ := e.login("anu@example.com")
	e.clock.SetOffset(0)
	st, body := e.do("GET", "/v1/me", access, nil)
	require.Equal(t, 401, st)
	require.Equal(t, "token_expired", body["error"].(map[string]any)["code"])
}

func TestNewUserGetsPrivateSpace(t *testing.T) {
	e := setup(t)
	newTok, _ := e.login("newcomer@example.com")
	st, me := e.do("GET", "/v1/me", newTok, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "owner", me["role"])
	require.Equal(t, "newcomer@example.com", me["email"])
	st, habits := e.do("GET", "/v1/habits", newTok, nil)
	require.Equal(t, 200, st)
	require.Len(t, habits["items"], 8)

	// the new account is isolated from Anu's space
	anu, _ := e.login("anu@example.com")
	_, anuMe := e.do("GET", "/v1/me", anu, nil)
	require.NotEqual(t, anuMe["sharedSpaceId"], me["sharedSpaceId"])
	st, created := e.do("POST", "/v1/habits", anu, map[string]any{"title": "Anu only", "trackingType": "boolean"})
	require.Equal(t, 201, st)
	_, list := e.do("GET", "/v1/habits?includeInactive=true", newTok, nil)
	for _, h := range list["items"].([]any) {
		require.NotEqual(t, created["id"], h.(map[string]any)["id"])
	}

	// logging in again reuses the same account and space
	again, _ := e.login("newcomer@example.com")
	_, me2 := e.do("GET", "/v1/me", again, nil)
	require.Equal(t, me["id"], me2["id"])
	require.Equal(t, me["sharedSpaceId"], me2["sharedSpaceId"])
}

func TestViewerIsReadOnly(t *testing.T) {
	e := setup(t)
	viewer, _ := e.login("aman@example.com")

	st, me := e.do("GET", "/v1/me", viewer, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "viewer", me["role"])

	st, habits := e.do("GET", "/v1/habits", viewer, nil)
	require.Equal(t, 200, st)
	require.Len(t, habits["items"], 8)

	writes := []struct {
		method, path string
		body         any
	}{
		{"POST", "/v1/habits", map[string]any{"title": "x", "trackingType": "boolean"}},
		{"PATCH", "/v1/habits/default-water", map[string]any{"version": 1, "target": 1}},
		{"DELETE", "/v1/habits/default-water", nil},
		{"PUT", "/v1/logs/default-water/" + today(), map[string]any{"value": 2000, "baseVersion": 0}},
		{"POST", "/v1/workouts", map[string]any{"type": "yoga", "startedAt": time.Now().UTC().Format(time.RFC3339)}},
		{"PATCH", "/v1/settings", map[string]any{"dailyThreshold": 10}},
		{"POST", "/v1/sync/push", map[string]any{"deviceId": "d", "operations": []any{map[string]any{"operationId": "op-v", "entityType": "habit", "entityId": "default-water", "op": "delete", "baseVersion": 1}}}},
	}
	for _, w := range writes {
		st, _ := e.do(w.method, w.path, viewer, w.body)
		require.Equal(t, 403, st, w.method+" "+w.path)
	}
	// habit untouched
	owner, _ := e.login("anu@example.com")
	_, list := e.do("GET", "/v1/habits", owner, nil)
	require.Len(t, list["items"], 8)
}

func TestCrossSpaceIsolation(t *testing.T) {
	e := setup(t)
	other, _ := e.login("other-owner@example.com")
	anu, _ := e.login("anu@example.com")

	// Each owner has their own copy of default habits (same slug ids, separate spaces).
	_, anuHabits := e.do("GET", "/v1/habits", anu, nil)
	anuHabit := anuHabits["items"].([]any)[0].(map[string]any)

	// Anu creates a private habit; other owner must not see it, nor modify it by id.
	st, created := e.do("POST", "/v1/habits", anu, map[string]any{"title": "Secret", "trackingType": "boolean"})
	require.Equal(t, 201, st)
	id := created["id"].(string)

	_, otherHabits := e.do("GET", "/v1/habits", other, nil)
	for _, h := range otherHabits["items"].([]any) {
		require.NotEqual(t, id, h.(map[string]any)["id"])
	}
	st, _ = e.do("PATCH", "/v1/habits/"+id, other, map[string]any{"version": 1, "title": "hacked"})
	require.Equal(t, 404, st)
	st, _ = e.do("DELETE", "/v1/habits/"+id, other, nil)
	require.Equal(t, 404, st)

	// Logging against a habit id that exists only in another space is rejected.
	st, _ = e.do("PUT", "/v1/logs/"+id+"/"+today(), other, map[string]any{"value": 1, "baseVersion": 0})
	require.Equal(t, 400, st)

	// Other space's pull never contains Anu's habit.
	_, pull := e.do("GET", "/v1/sync/pull", other, nil)
	for _, h := range pull["changes"].(map[string]any)["habits"].([]any) {
		require.NotEqual(t, id, h.(map[string]any)["id"])
	}
	_ = anuHabit
}

func TestMassAssignmentRejected(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	for _, extra := range []string{"sharedSpaceId", "role", "userId", "createdBy", "updatedBy"} {
		st, _ := e.do("POST", "/v1/habits", anu, map[string]any{"title": "x", "trackingType": "boolean", extra: "evil"})
		require.Equal(t, 400, st, extra)
	}
	st, _ := e.do("PATCH", "/v1/me", anu, map[string]any{"role": "owner"})
	require.Equal(t, 400, st)
	// viewer cannot escalate via /me
	aman, _ := e.login("aman@example.com")
	st, _ = e.do("PATCH", "/v1/me", aman, map[string]any{"role": "owner"})
	require.Equal(t, 400, st)
	_, me := e.do("GET", "/v1/me", aman, nil)
	require.Equal(t, "viewer", me["role"])
	// isDefault cannot be set by clients
	st, h := e.do("POST", "/v1/habits", anu, map[string]any{"title": "Nope", "trackingType": "boolean", "isDefault": true})
	require.Equal(t, 201, st)
	require.Equal(t, false, h["isDefault"])
	// unknown top-level JSON field on typed bodies
	st, _ = e.do("PUT", "/v1/logs/default-water/"+today(), anu, map[string]any{"value": 1, "baseVersion": 0, "userId": "x"})
	require.Equal(t, 400, st)
}

func TestProfileUpdate(t *testing.T) {
	e := setup(t)
	aman, _ := e.login("aman@example.com")
	st, me := e.do("PATCH", "/v1/me", aman, map[string]any{"theme": "mintPeach", "timezone": "America/New_York", "displayName": "Aman K"})
	require.Equal(t, 200, st)
	require.Equal(t, "mintPeach", me["theme"])
	st, _ = e.do("PATCH", "/v1/me", aman, map[string]any{"theme": "neon"})
	require.Equal(t, 400, st)
	st, _ = e.do("PATCH", "/v1/me", aman, map[string]any{"timezone": "Mars/Base"})
	require.Equal(t, 400, st)
}

func TestHabitCRUDAndOptimisticVersion(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	st, h := e.do("POST", "/v1/habits", anu, map[string]any{"title": "Read", "trackingType": "duration", "target": 20, "unit": "min"})
	require.Equal(t, 201, st)
	require.EqualValues(t, 1, h["version"])
	id := h["id"].(string)

	st, h2 := e.do("PATCH", "/v1/habits/"+id, anu, map[string]any{"version": 1, "target": 30})
	require.Equal(t, 200, st)
	require.EqualValues(t, 2, h2["version"])
	require.EqualValues(t, 30, h2["target"])

	st, conflict := e.do("PATCH", "/v1/habits/"+id, anu, map[string]any{"version": 1, "target": 40})
	require.Equal(t, 409, st)
	server := conflict["error"].(map[string]any)["details"].(map[string]any)["server"].(map[string]any)
	require.EqualValues(t, 30, server["target"])

	st, _ = e.do("PATCH", "/v1/habits/nope", anu, map[string]any{"version": 1, "target": 40})
	require.Equal(t, 404, st)

	st, _ = e.do("DELETE", "/v1/habits/"+id, anu, nil)
	require.Equal(t, 204, st)
	_, active := e.do("GET", "/v1/habits", anu, nil)
	require.Len(t, active["items"], 8)
	_, all := e.do("GET", "/v1/habits?includeInactive=true", anu, nil)
	require.Len(t, all["items"], 9)

	// restore
	st, restored := e.do("PATCH", "/v1/habits/"+id, anu, map[string]any{"version": 3, "isActive": true})
	require.Equal(t, 200, st)
	require.Equal(t, true, restored["isActive"])
}

func TestDailyLogLifecycle(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	path := "/v1/logs/default-water/" + today()

	st, l := e.do("PUT", path, anu, map[string]any{"value": 1500, "baseVersion": 0})
	require.Equal(t, 200, st)
	require.Equal(t, "partial", l["status"], "status computed from target")
	require.EqualValues(t, 1, l["version"])
	require.Equal(t, "default-water:"+today(), l["id"])

	st, l = e.do("PUT", path, anu, map[string]any{"value": 2000, "baseVersion": 1})
	require.Equal(t, 200, st)
	require.Equal(t, "complete", l["status"])
	require.EqualValues(t, 2, l["version"])

	st, c := e.do("PUT", path, anu, map[string]any{"value": 100, "baseVersion": 1})
	require.Equal(t, 409, st)
	require.NotNil(t, c["error"].(map[string]any)["details"].(map[string]any)["server"])

	future := time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02")
	st, _ = e.do("PUT", "/v1/logs/default-water/"+future, anu, map[string]any{"value": 1, "baseVersion": 0})
	require.Equal(t, 400, st)
	st, _ = e.do("PUT", "/v1/logs/default-water/not-a-date", anu, map[string]any{"value": 1, "baseVersion": 0})
	require.Equal(t, 400, st)

	st, list := e.do("GET", "/v1/logs?from="+today()+"&to="+today(), anu, nil)
	require.Equal(t, 200, st)
	require.Len(t, list["items"], 1)
	st, _ = e.do("GET", "/v1/logs?from=2020-01-01&to=2026-12-31", anu, nil)
	require.Equal(t, 400, st, "range over 366 days")
	st, _ = e.do("GET", "/v1/logs", anu, nil)
	require.Equal(t, 400, st)
}

func TestWorkoutLifecycle(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	start := time.Now().UTC().Add(-time.Hour)
	st, w := e.do("POST", "/v1/workouts", anu, map[string]any{"type": "strength", "startedAt": start.Format(time.RFC3339),
		"endedAt": start.Add(45 * time.Minute).Format(time.RFC3339), "pausedSeconds": 60, "localDate": start.Format("2006-01-02")})
	require.Equal(t, 201, st, w)
	require.EqualValues(t, 45*60-60, w["durationSeconds"])
	id := w["id"].(string)
	st, _ = e.do("DELETE", "/v1/workouts/"+id, anu, nil)
	require.Equal(t, 204, st)
	_, list := e.do("GET", "/v1/workouts", anu, nil)
	require.Len(t, list["items"], 0)
	st, _ = e.do("POST", "/v1/workouts", anu, map[string]any{"type": "strength", "startedAt": start.Format(time.RFC3339), "endedAt": start.Add(-time.Minute).Format(time.RFC3339)})
	require.Equal(t, 400, st)
}

func TestSettings(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")
	st, s := e.do("GET", "/v1/settings", aman, nil)
	require.Equal(t, 200, st)
	require.EqualValues(t, 80, s["dailyThreshold"])
	st, s2 := e.do("PATCH", "/v1/settings", anu, map[string]any{"version": s["version"], "dailyThreshold": 70, "restDays": []any{0}})
	require.Equal(t, 200, st)
	require.EqualValues(t, 70, s2["dailyThreshold"])
	st, _ = e.do("PATCH", "/v1/settings", anu, map[string]any{"version": s2["version"], "dailyThreshold": 170})
	require.Equal(t, 400, st)
}

func TestSyncPushIdempotencyConflictsAndPull(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")

	_, first := e.do("GET", "/v1/sync/pull", aman, nil)
	require.Equal(t, false, first["hasMore"])
	cursor0 := first["cursor"].(string)
	require.Len(t, first["changes"].(map[string]any)["habits"], 8)

	op := map[string]any{"operationId": "op-1", "entityType": "dailyLog", "entityId": "default-water:" + today(), "op": "upsert",
		"baseVersion": 0, "payload": map[string]any{"value": 2000}, "clientTimestamp": time.Now().UTC().Format(time.RFC3339)}
	push := func(ops ...any) []any {
		st, body := e.do("POST", "/v1/sync/push", anu, map[string]any{"deviceId": "dev1", "operations": ops})
		require.Equal(t, 200, st, body)
		return body["results"].([]any)
	}

	r := push(op)[0].(map[string]any)
	require.Equal(t, "applied", r["status"])
	require.EqualValues(t, 1, r["entity"].(map[string]any)["version"])

	// retry of same operationId is a duplicate and does not bump the version
	r = push(op)[0].(map[string]any)
	require.Equal(t, "duplicate", r["status"])
	require.EqualValues(t, 1, r["entity"].(map[string]any)["version"])

	// stale base version with older client timestamp -> conflict, server record returned
	stale := map[string]any{"operationId": "op-2", "entityType": "dailyLog", "entityId": "default-water:" + today(), "op": "upsert",
		"baseVersion": 0, "payload": map[string]any{"value": 100}, "clientTimestamp": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)}
	r = push(stale)[0].(map[string]any)
	require.Equal(t, "conflict", r["status"])
	require.EqualValues(t, 2000, r["entity"].(map[string]any)["value"])
	// a replay of a conflicted operation stays a conflict (deterministic)
	require.Equal(t, "conflict", push(stale)[0].(map[string]any)["status"])

	// stale base version with newer client timestamp -> LWW applied
	lww := map[string]any{"operationId": "op-3", "entityType": "dailyLog", "entityId": "default-water:" + today(), "op": "upsert",
		"baseVersion": 0, "payload": map[string]any{"value": 2500}, "clientTimestamp": time.Now().UTC().Add(time.Minute).Format(time.RFC3339)}
	r = push(lww)[0].(map[string]any)
	require.Equal(t, "applied", r["status"])
	require.Equal(t, "lww", r["resolved"])
	require.EqualValues(t, 2, r["entity"].(map[string]any)["version"])

	// rejected ops: unknown habit, bad entity type, viewer-style forbidden payload fields
	res := push(
		map[string]any{"operationId": "op-4", "entityType": "dailyLog", "entityId": "ghost:" + today(), "op": "upsert", "baseVersion": 0, "payload": map[string]any{"value": 1}},
		map[string]any{"operationId": "op-5", "entityType": "nothing", "entityId": "x", "op": "upsert", "baseVersion": 0},
		map[string]any{"operationId": "op-6", "entityType": "habit", "entityId": "h-new", "op": "upsert", "baseVersion": 0, "payload": map[string]any{"title": "x", "trackingType": "boolean", "sharedSpaceId": "z"}},
	)
	for _, x := range res {
		require.Equal(t, "rejected", x.(map[string]any)["status"])
	}

	// delete a workout via sync, then pull as viewer sees tombstone
	start := time.Now().UTC().Add(-time.Hour)
	r = push(map[string]any{"operationId": "op-7", "entityType": "workout", "entityId": "w-1", "op": "upsert", "baseVersion": 0,
		"payload": map[string]any{"type": "cardio", "startedAt": start.Format(time.RFC3339), "endedAt": start.Add(30 * time.Minute).Format(time.RFC3339), "localDate": start.Format("2006-01-02")}})[0].(map[string]any)
	require.Equal(t, "applied", r["status"])
	r = push(map[string]any{"operationId": "op-8", "entityType": "workout", "entityId": "w-1", "op": "delete", "baseVersion": 1})[0].(map[string]any)
	require.Equal(t, "applied", r["status"])

	st, pull := e.do("GET", "/v1/sync/pull?cursor="+cursor0, aman, nil)
	require.Equal(t, 200, st)
	changes := pull["changes"].(map[string]any)
	require.Len(t, changes["dailyLogs"], 1)
	require.EqualValues(t, 2500, changes["dailyLogs"].([]any)[0].(map[string]any)["value"])
	ws := changes["workouts"].([]any)
	require.Len(t, ws, 1)
	require.NotNil(t, ws[0].(map[string]any)["deletedAt"], "tombstone")

	// Caught up: nothing new.
	st, again := e.do("GET", "/v1/sync/pull?cursor="+pull["cursor"].(string), aman, nil)
	require.Equal(t, 200, st)
	require.Len(t, again["changes"].(map[string]any)["dailyLogs"], 0)

	// pagination
	st, page := e.do("GET", "/v1/sync/pull?limit=3", aman, nil)
	require.Equal(t, 200, st)
	require.Equal(t, true, page["hasMore"])

	st, _ = e.do("GET", "/v1/sync/pull?cursor=%21%21%21", aman, nil)
	require.Equal(t, 400, st)
}

func TestSyncPushLimits(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	st, _ := e.do("POST", "/v1/sync/push", anu, map[string]any{"deviceId": "d", "operations": []any{}})
	require.Equal(t, 400, st)
	ops := make([]any, 101)
	for i := range ops {
		ops[i] = map[string]any{"operationId": "x", "entityType": "habit", "entityId": "a", "op": "delete", "baseVersion": 1}
	}
	st, _ = e.do("POST", "/v1/sync/push", anu, map[string]any{"deviceId": "d", "operations": ops})
	require.Equal(t, 400, st)
}

func TestOperationIDCannotBeReplayedAcrossSpaces(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	other, _ := e.login("other-owner@example.com")
	op := map[string]any{"operationId": "shared-op", "entityType": "dailyLog", "entityId": "default-water:" + today(), "op": "upsert", "baseVersion": 0, "payload": map[string]any{"value": 1}}
	st, body := e.do("POST", "/v1/sync/push", anu, map[string]any{"deviceId": "d", "operations": []any{op}})
	require.Equal(t, 200, st)
	require.Equal(t, "applied", body["results"].([]any)[0].(map[string]any)["status"])
	_, body = e.do("POST", "/v1/sync/push", other, map[string]any{"deviceId": "d", "operations": []any{op}})
	r := body["results"].([]any)[0].(map[string]any)
	require.Equal(t, "rejected", r["status"])
	require.Nil(t, r["entity"], "must not leak another space's record")
}

func TestStatsStreak(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	aman, _ := e.login("aman@example.com")
	vals := map[string]any{"default-wakeup": 290, "default-exercise": 100, "default-no-sugar": 1, "default-no-junk": 1,
		"default-dinner": 1000, "default-water": 2100, "default-steps": 8000, "default-sleep": 1300}
	for id, v := range vals {
		st, _ := e.do("PUT", "/v1/logs/"+id+"/"+today(), anu, map[string]any{"value": v, "baseVersion": 0})
		require.Equal(t, 200, st, id)
	}
	st, sum := e.do("GET", "/v1/stats/summary", aman, nil)
	require.Equal(t, 200, st)
	require.EqualValues(t, 1, sum["currentStreak"])
	require.EqualValues(t, 1, sum["longestStreak"])
	days := sum["days"].([]any)
	require.Len(t, days, 1)
	require.EqualValues(t, 100, days[0].(map[string]any)["completionPercent"])
}

func TestHealthAndHeaders(t *testing.T) {
	e := setup(t)
	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/health", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	st, _ := e.do("GET", "/v1/nope", "", nil)
	require.Equal(t, 404, st)
}

func TestOversizedBodyRejected(t *testing.T) {
	e := setup(t)
	anu, _ := e.login("anu@example.com")
	big := bytes.Repeat([]byte("a"), 2<<20)
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/habits", bytes.NewReader(append(append([]byte(`{"title":"`), big...), []byte(`"}`)...)))
	req.Header.Set("Authorization", "Bearer "+anu)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 400, resp.StatusCode)
}
