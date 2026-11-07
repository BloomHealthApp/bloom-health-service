package httpserver_test

import (
	"encoding/json"
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

	resp, body = raw(t, e, "DELETE", "/v1/auth/signin/request", "", "")
	require.Equal(t, 405, resp.StatusCode)
	require.JSONEq(t, `{"error":{"code":"validation_failed","message":"method not allowed"}}`, body)

	// strict JSON: unknown fields, trailing data and empty bodies are 400 validation_failed
	for _, b := range []string{`{"identifier":"anu@example.com","x":1}`, `{"identifier":"anu@example.com"} {}`, ``, `not json`} {
		resp, body = raw(t, e, "POST", "/v1/auth/signin/request", "", b)
		require.Equal(t, 400, resp.StatusCode, b)
		require.Contains(t, body, `"code":"validation_failed"`)
	}
	for _, b := range []string{`{"identifier":"not-an-email@"}`, `{"identifier":"12345"}`, `{"identifier":""}`} {
		resp, _ = raw(t, e, "POST", "/v1/auth/signin/request", "", b)
		require.Equal(t, 400, resp.StatusCode, b)
	}
	resp, body = raw(t, e, "POST", "/v1/auth/signup/request", "", `{"name":"A","phone":"9876543210","email":"a@b.io","role":"owner"}`)
	require.Equal(t, 400, resp.StatusCode)
	require.Contains(t, body, `"code":"validation_failed"`)
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

func TestPrivacyPolicyPage(t *testing.T) {
	e := setup(t)
	resp, body := raw(t, e, "GET", "/privacy", "", "")
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	require.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'", resp.Header.Get("Content-Security-Policy"), "styles only, still no scripts")
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	require.Contains(t, body, "<title>Bloom Health privacy policy</title>")
	require.Contains(t, body, "<h2>Keeping and deleting your data</h2>")
	require.Contains(t, body, `<a href="mailto:support.bloomhealth@gmail.com">support.bloomhealth@gmail.com</a>`)
	require.Contains(t, body, "<li>Account details:")
	require.NotContains(t, body, "<script")

	resp, _ = raw(t, e, "POST", "/privacy", "", "")
	require.Equal(t, 405, resp.StatusCode)
}

func TestDeleteAccountPage(t *testing.T) {
	e := setup(t)
	resp, body := raw(t, e, "GET", "/delete-account", "", "")
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	require.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'", resp.Header.Get("Content-Security-Policy"))
	// What Google Play asks of the page: the app name, the steps, and what is deleted or kept and for how long.
	require.Contains(t, body, "<title>Delete your Bloom Health account</title>")
	require.Contains(t, body, "<h2>How to delete your account</h2>")
	require.Contains(t, body, "Settings → Delete account")
	require.Contains(t, body, "For 15 days")
	require.Contains(t, body, "<h2>What is deleted</h2>")
	require.Contains(t, body, "<h2>What is kept</h2>")
	require.Contains(t, body, `<a href="mailto:support.bloomhealth@gmail.com">support.bloomhealth@gmail.com</a>`)
	require.Contains(t, body, `<a href="https://bloom-health-service.vercel.app/privacy">`)
	require.NotContains(t, body, "<script")
}

func TestPerIdentifierRateLimitDoesNotRevealAndVerifyReturns429(t *testing.T) {
	e := setup(t)
	// signin request: burst of 3 per identifier, further requests still answer 202 (decoy challenge)
	// but send nothing new
	for i := 0; i < 6; i++ {
		resp, body := raw(t, e, "POST", "/v1/auth/signin/request", "", `{"identifier":"anu@example.com"}`)
		require.Equal(t, 202, resp.StatusCode)
		var parsed map[string]string
		require.NoError(t, json.Unmarshal([]byte(body), &parsed))
		require.Equal(t, "sent", parsed["status"])
		require.NotEmpty(t, parsed["challengeId"])
		require.Len(t, parsed, 2)
	}
	require.Len(t, e.mail.Messages, 3, "mails stop at the per-identifier burst")
}

func TestSignUpPerIdentifierRateLimitReturns429(t *testing.T) {
	e := setup(t)
	// sign-up already reveals registration (409), so over-limit is an honest 429
	var sr *http.Response
	for i := 0; i < 4; i++ {
		sr, _ = raw(t, e, "POST", "/v1/auth/signup/request", "", `{"name":"A","phone":"9111111111","email":"fresh@example.com"}`)
	}
	require.Equal(t, 429, sr.StatusCode)
	require.Equal(t, "60", sr.Header.Get("Retry-After"))
}

func TestVerifyPerChallengeRateLimitReturns429(t *testing.T) {
	e := setup(t)
	// verify: burst of 3 per challenge, then 429 with Retry-After
	var last *http.Response
	for i := 0; i < 4; i++ {
		last, _ = raw(t, e, "POST", "/v1/auth/otp/verify", "", `{"challengeId":"some-challenge","code":"000000"}`)
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
