package request

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignUpValidateAndNormalize(t *testing.T) {
	good := SignUpReq{Name: "  Anu  ", Phone: " 98765 43210 ", Email: "Anu@Example.com "}
	require.Nil(t, good.Validate())
	require.Nil(t, good.Normalize("+91"))
	require.Equal(t, SignUpReq{Name: "Anu", Phone: "+919876543210", Email: "anu@example.com"}, good)

	bad := map[string]SignUpReq{
		"name must be 1-40 characters":                                       {Name: " ", Phone: "9876543210", Email: "a@b.io"},
		"name must be 1-40 characters ":                                      {Name: strings.Repeat("x", 41), Phone: "9876543210", Email: "a@b.io"},
		"a valid email is required":                                          {Name: "A", Phone: "9876543210", Email: "nope"},
		"a valid email is required ":                                         {Name: "A", Phone: "9876543210", Email: strings.Repeat("a", 250) + "@x.io"},
		"phone must be a valid number (8-15 digits, optional +country code)": {Name: "A", Phone: "", Email: "a@b.io"},
	}
	for want, r := range bad {
		herr := r.Validate()
		require.NotNil(t, herr, want)
		require.Equal(t, strings.TrimSpace(want), herr.Error())
	}
	badPhone := SignUpReq{Name: "A", Phone: "12345", Email: "a@b.io"}
	require.Nil(t, badPhone.Validate())
	require.NotNil(t, badPhone.Normalize("+91"))
}

func TestSignInNormalize(t *testing.T) {
	cases := []struct {
		in, want string
		isEmail  bool
		ok       bool
	}{
		{" Anu@Example.com ", "anu@example.com", true, true},
		{"98765 43210", "+919876543210", false, true},
		{"+1 (415) 555-2671", "+14155552671", false, true},
		{"not-a-number", "", false, false},
		{"bad@", "", false, false},
	}
	for _, tc := range cases {
		r := SignInReq{Identifier: tc.in}
		require.Nil(t, r.Validate(), tc.in)
		herr := r.Normalize("+91")
		require.Equal(t, tc.ok, herr == nil, tc.in)
		if tc.ok {
			require.Equal(t, tc.want, r.Identifier)
			require.Equal(t, tc.isEmail, r.IsEmail)
		}
	}
	empty := SignInReq{Identifier: "  "}
	require.NotNil(t, empty.Validate())
}

func TestPatchMeValidate(t *testing.T) {
	cases := map[string]string{
		`{}`: "",
		`{"displayName":"Aman K","theme":"mintPeach","timezone":"Asia/Kolkata","avatar":null}`: "",
		`{"displayName":""}`:       "displayName must be 1-40 characters",
		`{"displayName":null}`:     "displayName must be 1-40 characters",
		`{"theme":"forestTrail"}`:  "",
		`{"theme":"midnightNavy"}`: "",
		`{"theme":"sunsetEmber"}`:  "",
		`{"theme":"graphiteGrey"}`: "",
		`{"theme":"mintPeach"}`:    "",
		`{"theme":"neon"}`:         "invalid theme",
		`{"timezone":"Mars/Base"}`: "timezone must be a valid IANA zone",
		`{"timezone":"Local"}`:     "timezone must be a valid IANA zone",
	}
	for body, want := range cases {
		var r PatchMeReq
		require.NoError(t, json.Unmarshal([]byte(body), &r), body)
		herr := r.Validate()
		if want == "" {
			require.Nil(t, herr, body)
		} else {
			require.NotNil(t, herr, body)
			require.Equal(t, want, herr.Error(), body)
		}
	}
}

func TestRangeValidate(t *testing.T) {
	bad := []RangeReq{{Required: true}, {From: "2026-1-1"}, {To: "x"}, {From: "2026-02-01", To: "2026-01-01"}, {From: "2020-01-01", To: "2026-12-31"}}
	for _, r := range bad {
		require.NotNil(t, r.Validate(), r)
	}
	ok := RangeReq{From: "2026-01-01", To: "2026-01-31", Required: true}
	require.Nil(t, ok.Validate())
}

func TestSyncPullAndPushValidate(t *testing.T) {
	r := SyncPullReq{RawLimit: "12x"}
	require.NotNil(t, r.Validate())
	r = SyncPullReq{Cursor: "!!!"}
	require.NotNil(t, r.Validate())
	r = SyncPullReq{RawLimit: "25"}
	require.Nil(t, r.Validate())
	require.Equal(t, 25, r.Limit)
	require.NotNil(t, (&SyncPushReq{}).Validate())
	require.NotNil(t, (&SyncPushReq{Operations: make([]PushOp, 101)}).Validate())
	require.Nil(t, (&SyncPushReq{Operations: make([]PushOp, 100)}).Validate())
}

func TestPatchEntityVersionExtraction(t *testing.T) {
	r := PatchEntityReq{Payload: map[string]any{"version": 3.0, "id": "x", "title": "t"}}
	require.Nil(t, r.Validate())
	require.True(t, r.HasVersion)
	require.Equal(t, 3, r.Version)
	require.Equal(t, map[string]any{"title": "t"}, r.Payload)
	r = PatchEntityReq{Payload: map[string]any{"version": "3"}}
	_ = r.Validate()
	require.False(t, r.HasVersion)
}

func TestPatchMeAvatarValidate(t *testing.T) {
	const msg = "avatar must be null or one of bubu-01 to bubu-15"
	ok := []string{`{"avatar":null}`, `{"avatar":"bubu-01"}`, `{"avatar":"bubu-15"}`}
	for _, body := range ok {
		var r PatchMeReq
		require.NoError(t, json.Unmarshal([]byte(body), &r), body)
		require.Nil(t, r.Validate(), body)
	}
	bad := []string{`{"avatar":""}`, `{"avatar":"bubu-00"}`, `{"avatar":"bubu-16"}`, `{"avatar":"Bubu-01"}`, `{"avatar":"bubu-01 "}`,
		`{"avatar":"https://x/y.png"}`, `{"avatar":"` + strings.Repeat("a", 500) + `"}`}
	for _, body := range bad {
		var r PatchMeReq
		require.NoError(t, json.Unmarshal([]byte(body), &r), body)
		herr := r.Validate()
		require.NotNil(t, herr, body)
		require.Equal(t, msg, herr.Error(), body)
	}
	var r PatchMeReq
	require.Error(t, json.Unmarshal([]byte(`{"avatar":5}`), &r))
}

func TestSignUpReqAvatarValidate(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, a := range []*string{nil, str("bubu-01"), str("bubu-15")} {
		r := SignUpReq{Name: "Riya", Phone: "9876543210", Email: "r@example.com", Avatar: a}
		require.Nil(t, r.Validate())
	}
	for _, a := range []*string{str(""), str("bubu-16"), str("evil"), str(strings.Repeat("a", 300))} {
		r := SignUpReq{Name: "Riya", Phone: "9876543210", Email: "r@example.com", Avatar: a}
		herr := r.Validate()
		require.NotNil(t, herr, *a)
		require.Contains(t, herr.Error(), "avatar must be null or one of bubu-01 to bubu-15")
	}
}
