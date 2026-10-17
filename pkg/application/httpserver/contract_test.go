package httpserver_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func raw(t *testing.T, e *env, method, path, token, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestErrorEnvelopeAndStatusCodes(t *testing.T) {
	e := setup(t)

	resp, body := raw(t, e, "GET", "/v1/me", "", "")
	require.Equal(t, 401, resp.StatusCode)
	require.JSONEq(t, `{"error":{"code":"unauthorized","message":"missing bearer token"}}`, body)
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"), "secure headers also on errors")
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	require.NotEmpty(t, resp.Header.Get("X-Request-Id"))

	resp, body = raw(t, e, "GET", "/v1/nope", "", "")
	require.Equal(t, 404, resp.StatusCode)
	require.JSONEq(t, `{"error":{"code":"not_found","message":"no such route"}}`, body)

	resp, body = raw(t, e, "DELETE", "/v1/auth/otp/request", "", "")
	require.Equal(t, 405, resp.StatusCode)
	require.JSONEq(t, `{"error":{"code":"validation_failed","message":"method not allowed"}}`, body)

	// strict JSON: unknown fields, trailing data and empty bodies are 400 validation_failed
	for _, b := range []string{`{"email":"anu@example.com","x":1}`, `{"email":"anu@example.com"} {}`, ``, `not json`} {
		resp, body = raw(t, e, "POST", "/v1/auth/otp/request", "", b)
		require.Equal(t, 400, resp.StatusCode, b)
		require.Contains(t, body, `"code":"validation_failed"`)
	}
	resp, _ = raw(t, e, "POST", "/v1/auth/otp/request", "", `{"email":"not-an-email"}`)
	require.Equal(t, 400, resp.StatusCode)
}

func TestHealthEndpoints(t *testing.T) {
	e := setup(t)
	for _, p := range []string{"/health", "/v1/health"} {
		resp, body := raw(t, e, "GET", p, "", "")
		require.Equal(t, 200, resp.StatusCode, p)
		require.Contains(t, body, `"status":"ok"`)
		require.Contains(t, body, `"version":"1.0.0"`)
	}
}

func TestPerEmailRateLimitDoesNotRevealAndVerifyReturns429(t *testing.T) {
	e := setup(t)
	// request: burst of 3 per email, further requests still answer 202 but send nothing new
	for i := 0; i < 6; i++ {
		resp, body := raw(t, e, "POST", "/v1/auth/otp/request", "", `{"email":"anu@example.com"}`)
		require.Equal(t, 202, resp.StatusCode)
		require.JSONEq(t, `{"status":"sent"}`, body)
	}
	// verify: burst of 3 per email, then 429 with Retry-After
	var last *http.Response
	for i := 0; i < 4; i++ {
		last, _ = raw(t, e, "POST", "/v1/auth/otp/verify", "", `{"email":"anu@example.com","code":"000000"}`)
	}
	require.Equal(t, 429, last.StatusCode)
	require.Equal(t, "60", last.Header.Get("Retry-After"))
}

func TestAuthRouteIPRateLimit(t *testing.T) {
	e := setup(t)
	var last *http.Response
	var body string
	for i := 0; i < 12; i++ { // burst 10 on /auth/*
		last, body = raw(t, e, "POST", "/v1/auth/logout", "", `{"refreshToken":"x"}`)
	}
	require.Equal(t, 429, last.StatusCode)
	require.Equal(t, "30", last.Header.Get("Retry-After"))
	require.JSONEq(t, `{"error":{"code":"rate_limited","message":"too many attempts, try again later"}}`, body)
}

func TestLogoutRevokesSession(t *testing.T) {
	e := setup(t)
	_, refresh := e.login("anu@example.com")
	st, _ := e.do("POST", "/v1/auth/logout", "", map[string]any{"refreshToken": refresh})
	require.Equal(t, 204, st)
	st, _ = e.do("POST", "/v1/auth/refresh", "", map[string]any{"refreshToken": refresh})
	require.Equal(t, 401, st)
	st, _ = e.do("POST", "/v1/auth/logout", "", map[string]any{"refreshToken": "unknown"})
	require.Equal(t, 204, st, "unknown tokens are not revealed")
}

func TestRoleIsResolvedFromDatabaseEachRequest(t *testing.T) {
	e := setup(t)
	aman, _ := e.login("aman@example.com")
	st, _ := e.do("POST", "/v1/habits", aman, map[string]any{"title": "x", "trackingType": "boolean"})
	require.Equal(t, 403, st)
	// Promote the viewer in the database: the same token is now an owner on the next request.
	ctx := t.Context()
	u, err := e.repos.Users.ByEmail(ctx, "aman@example.com")
	require.NoError(t, err)
	m, err := e.repos.Memberships.ForUser(ctx, u.ID)
	require.NoError(t, err)
	m.Role = "owner"
	require.NoError(t, e.repos.Memberships.Save(ctx, m))
	st, _ = e.do("POST", "/v1/habits", aman, map[string]any{"title": "x", "trackingType": "boolean"})
	require.Equal(t, 201, st)
}
