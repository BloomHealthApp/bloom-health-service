package httpserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/ratelimit"
)

// relaxLimits lifts the per-IP/per-identifier auth limits (covered by contract_test.go) so a test can
// make many auth calls.
func (e *env) relaxLimits() {
	l := di.AppLogic.GetRateLimiters()
	l.Auth = ratelimit.NewKeyed(rate.Inf, 100000)
	l.Email = ratelimit.NewKeyed(rate.Inf, 100000)
}

func errCode(body map[string]any) string {
	if m, ok := body["error"].(map[string]any); ok {
		s, _ := m["code"].(string)
		return s
	}
	return ""
}

func TestSignUpHappyPath(t *testing.T) {
	e := setup(t)
	st, resp := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "  Riya Sen ", "phone": "98765 43210", "email": "Riya@Example.com"})
	require.Equal(t, 202, st)
	require.Equal(t, "sent", resp["status"])
	require.NotEmpty(t, resp["challengeId"])
	require.Len(t, resp, 2)

	require.Len(t, e.mail.Messages, 1)
	msg := e.mail.Messages[0]
	require.Equal(t, "riya@example.com", msg.To)
	require.Contains(t, msg.HTML, "Welcome, Riya Sen!")
	require.NotContains(t, msg.HTML, "Welcome back")
	ctx := context.Background()
	_, err := e.repos.Users.ByEmail(ctx, "riya@example.com")
	require.Error(t, err, "no account until the code is verified")
	_, err = e.repos.Users.ByPhone(ctx, "+919876543210")
	require.Error(t, err)

	code := codeRe.FindString(msg.Body)
	st, body := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": resp["challengeId"], "code": code})
	require.Equal(t, 200, st, body)
	require.EqualValues(t, 900, body["expiresIn"])
	user := body["user"].(map[string]any)
	require.Equal(t, "riya@example.com", user["email"])
	require.Equal(t, "+919876543210", user["phone"])
	require.Equal(t, "Riya Sen", user["displayName"])
	require.Equal(t, "owner", user["role"])
	require.Equal(t, "Riya Sen", user["ownerDisplayName"])

	access := body["accessToken"].(string)
	st, me := e.do("GET", "/v1/me", access, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "+919876543210", me["phone"])
	st, habits := e.do("GET", "/v1/habits", access, nil)
	require.Equal(t, 200, st)
	require.Len(t, habits["items"], 8)
	st, settings := e.do("GET", "/v1/settings", access, nil)
	require.Equal(t, 200, st)
	require.EqualValues(t, 80, settings["dailyThreshold"])

	// the private space is separate from Anu's seeded space
	anu, _ := e.login("anu@example.com")
	_, anuMe := e.do("GET", "/v1/me", anu, nil)
	require.NotEqual(t, anuMe["sharedSpaceId"], me["sharedSpaceId"])

	// stored user has the E.164 phone
	u, err := e.repos.Users.ByEmail(ctx, "riya@example.com")
	require.NoError(t, err)
	require.NotNil(t, u.Phone)
	require.Equal(t, "+919876543210", *u.Phone)
}

func TestSignUpEscapesNameInEmail(t *testing.T) {
	e := setup(t)
	st, _ := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": `<img src=x onerror=alert(1)>`, "phone": "9876543210", "email": "x@example.com"})
	require.Equal(t, 202, st)
	require.Len(t, e.mail.Messages, 1)
	require.NotContains(t, e.mail.Messages[0].HTML, "<img")
	require.Contains(t, e.mail.Messages[0].HTML, "&lt;img")
}

func TestSignUpDuplicateEmailOrPhoneIs409(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	e.signup("Riya", "9876543210", "riya@example.com")
	e.mail.Messages = nil

	cases := []struct {
		name  string
		body  map[string]any
		descr string
	}{
		{"seeded legacy email", map[string]any{"name": "X", "phone": "9000000009", "email": "anu@example.com"}, "email taken"},
		{"same email different case", map[string]any{"name": "X", "phone": "9000000009", "email": "RIYA@example.com"}, "email taken"},
		{"same phone bare", map[string]any{"name": "X", "phone": "9876543210", "email": "new1@example.com"}, "phone taken"},
		{"same phone formatted", map[string]any{"name": "X", "phone": "+91 (98765) 43-210", "email": "new2@example.com"}, "phone taken"},
	}
	for _, tc := range cases {
		st, body := e.do("POST", "/v1/auth/signup/request", "", tc.body)
		require.Equal(t, 409, st, tc.name)
		require.Equal(t, "already_registered", errCode(body), tc.name)
	}
	require.Empty(t, e.mail.Messages, "no code is mailed for a taken email/phone")
}

func TestSignUpPhoneNormalizationAndValidation(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	body := e.signup("Intl", "+1 (415) 555-2671", "intl@example.com")
	require.Equal(t, "+14155552671", body["user"].(map[string]any)["phone"])

	for _, bad := range []map[string]any{
		{"name": "A", "phone": "12345", "email": "a@example.com"},
		{"name": "A", "phone": "98765abcde", "email": "a@example.com"},
		{"name": "A", "phone": "", "email": "a@example.com"},
		{"name": "A", "phone": "9876543210", "email": "nope"},
		{"name": "", "phone": "9876543210", "email": "a@example.com"},
		{"name": "012345678901234567890123456789012345678901", "phone": "9876543210", "email": "a@example.com"},
		{"name": "A", "email": "a@example.com"},
	} {
		st, resp := e.do("POST", "/v1/auth/signup/request", "", bad)
		require.Equal(t, 400, st, bad)
		require.Equal(t, "validation_failed", errCode(resp))
	}
}

func TestSignInByPhoneMailsTheMappedEmail(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	first := e.signup("Riya", "9876543210", "riya@example.com")
	firstUser := first["user"].(map[string]any)

	for _, id := range []string{"9876543210", "+91 98765-43210", "+919876543210", "RIYA@example.com"} {
		e.mail.Messages = nil
		st, resp := e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": id})
		require.Equal(t, 202, st, id)
		require.Equal(t, "sent", resp["status"])
		require.Len(t, e.mail.Messages, 1, id)
		require.Equal(t, "riya@example.com", e.mail.Messages[0].To, "the code goes to the email on file, never to the phone")
		require.Contains(t, e.mail.Messages[0].HTML, "Welcome back")
		code := codeRe.FindString(e.mail.Messages[0].Body)
		st, body := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": resp["challengeId"], "code": code})
		require.Equal(t, 200, st, body)
		require.Equal(t, firstUser["id"], body["user"].(map[string]any)["id"], "same account")
		require.Equal(t, firstUser["sharedSpaceId"], body["user"].(map[string]any)["sharedSpaceId"])
	}
}

func TestSignInUnknownIdentifierGetsDecoyAndNoMail(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	e.signup("Riya", "9876543210", "riya@example.com")
	e.mail.Messages = nil

	var decoys []string
	for _, id := range []string{"9111111111", "+14155550000", "ghost@example.com"} {
		st, resp := e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": id})
		require.Equal(t, 202, st, id)
		require.Equal(t, "sent", resp["status"])
		require.Len(t, resp, 2, "same shape as a real response")
		cid, _ := resp["challengeId"].(string)
		require.NotEmpty(t, cid)
		decoys = append(decoys, cid)
	}
	require.Empty(t, e.mail.Messages, "nothing is mailed for unknown identifiers")
	// A real challenge looks the same (opaque string of the same length).
	_, real := e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": "riya@example.com"})
	require.Len(t, real["challengeId"], len(decoys[0]))

	// decoys never verify, whatever code is tried
	for _, cid := range decoys {
		for _, code := range []string{"000000", "123456", codeRe.FindString(e.mail.Messages[0].Body)} {
			st, _ := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cid, "code": code})
			require.Equal(t, 401, st)
		}
	}
	_, err := e.repos.Users.ByEmail(context.Background(), "ghost@example.com")
	require.Error(t, err, "signin never creates accounts")
}

func TestChallengeSingleUseAttemptLimitAndExpiry(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	ctx := context.Background()
	svc := di.AppLogic.GetAuthService()

	signin := func() (cid, code string) {
		e.mail.Messages = nil
		st, resp := e.do("POST", "/v1/auth/signin/request", "", map[string]any{"identifier": "anu@example.com"})
		require.Equal(t, 202, st)
		return resp["challengeId"].(string), codeRe.FindString(e.mail.Messages[0].Body)
	}

	// single use
	cid, code := signin()
	st, _ := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cid, "code": code})
	require.Equal(t, 200, st)
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cid, "code": code})
	require.Equal(t, 401, st)

	// attempt limit: five wrong guesses burn the challenge, the right code no longer works
	cid, code = signin()
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: cid, Code: wrong})
		require.NotNil(t, herr)
	}
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cid, "code": code})
	require.Equal(t, 401, st)

	// expiry: valid code after the 10 minute TTL
	cid, code = signin()
	e.clock.SetOffset(11 * time.Minute)
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cid, "code": code})
	require.Equal(t, 401, st)
	e.clock.SetOffset(0)

	// a newer code retires the older one for the same account
	cid1, code1 := signin()
	cid2, code2 := signin()
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cid1, "code": code1})
	require.Equal(t, 401, st, "older challenge is retired")
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cid2, "code": code2})
	require.Equal(t, 200, st)

	// a code does not work on another challenge
	cidA, codeA := signin()
	cidB, _ := signin()
	_ = cidA
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": cidB, "code": codeA})
	require.Equal(t, 401, st)
}

func TestSignUpChallengeCannotBeUsedAfterExpiryAndCreatesNothing(t *testing.T) {
	e := setup(t)
	st, resp := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "Late", "phone": "9333333333", "email": "late@example.com"})
	require.Equal(t, 202, st)
	code := codeRe.FindString(e.mail.Messages[0].Body)
	e.clock.SetOffset(11 * time.Minute)
	st, _ = e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": resp["challengeId"], "code": code})
	require.Equal(t, 401, st)
	_, err := e.repos.Users.ByEmail(context.Background(), "late@example.com")
	require.Error(t, err)
}

func TestSignUpRaceOnPhoneAtVerifyIs409(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	// two sign-ups for the same phone are requested before either is verified
	_, r1 := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "One", "phone": "9444444444", "email": "one@example.com"})
	c1 := codeRe.FindString(e.mail.Messages[0].Body)
	_, r2 := e.do("POST", "/v1/auth/signup/request", "", map[string]any{"name": "Two", "phone": "9444444444", "email": "two@example.com"})
	c2 := codeRe.FindString(e.mail.Messages[1].Body)
	st, _ := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": r1["challengeId"], "code": c1})
	require.Equal(t, 200, st)
	st, body := e.do("POST", "/v1/auth/otp/verify", "", map[string]any{"challengeId": r2["challengeId"], "code": c2})
	require.Equal(t, 409, st)
	require.Equal(t, "already_registered", errCode(body))
	_, err := e.repos.Users.ByEmail(context.Background(), "two@example.com")
	require.Error(t, err, "the losing sign-up creates no account")
}

func TestLegacyUserWithoutPhoneSignsInByEmail(t *testing.T) {
	e := setup(t)
	access, _ := e.login("anu@example.com") // seeded legacy account: no phone
	st, me := e.do("GET", "/v1/me", access, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "anu@example.com", me["email"])
	phone, present := me["phone"]
	require.True(t, present, "phone key is always present")
	require.Nil(t, phone, "legacy accounts have a null phone")
	require.Equal(t, "owner", me["role"])
}

func TestOldOTPRequestEndpointIsGone(t *testing.T) {
	e := setup(t)
	st, body := e.do("POST", "/v1/auth/otp/request", "", map[string]any{"email": "anu@example.com"})
	require.Equal(t, 404, st)
	require.Equal(t, "not_found", errCode(body))
	require.Empty(t, e.mail.Messages)
}

func TestPatchMePhone(t *testing.T) {
	e := setup(t)
	e.relaxLimits()
	e.signup("Riya", "9876543210", "riya@example.com")
	anu, _ := e.login("anu@example.com") // legacy: no phone yet

	// legacy user adds a phone; it is normalized
	st, me := e.do("PATCH", "/v1/me", anu, map[string]any{"phone": "98 111-22222"})
	require.Equal(t, 200, st)
	require.Equal(t, "+919811122222", me["phone"])
	// ...and can then sign in with it
	e.mail.Messages = nil
	acc, _ := e.login("9811122222")
	_, me2 := e.do("GET", "/v1/me", acc, nil)
	require.Equal(t, me["id"], me2["id"])

	// taken by another account (any formatting) -> 409 already_registered, profile unchanged
	st, body := e.do("PATCH", "/v1/me", anu, map[string]any{"phone": "+91 98765 43210"})
	require.Equal(t, 409, st)
	require.Equal(t, "already_registered", errCode(body))
	_, me3 := e.do("GET", "/v1/me", anu, nil)
	require.Equal(t, "+919811122222", me3["phone"])

	// re-sending your own number is fine; other fields still patch alongside
	st, me4 := e.do("PATCH", "/v1/me", anu, map[string]any{"phone": "+919811122222", "displayName": "Anu K"})
	require.Equal(t, 200, st)
	require.Equal(t, "Anu K", me4["displayName"])

	// invalid values
	for _, bad := range []any{"123", "abcdefghij", "", nil} {
		st, _ = e.do("PATCH", "/v1/me", anu, map[string]any{"phone": bad})
		require.Equal(t, 400, st, bad)
	}
	// omitting phone leaves it untouched
	st, me5 := e.do("PATCH", "/v1/me", anu, map[string]any{"theme": "mintPeach"})
	require.Equal(t, 200, st)
	require.Equal(t, "+919811122222", me5["phone"])
}
