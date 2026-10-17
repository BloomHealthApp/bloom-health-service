package request

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOTPRequestValidate(t *testing.T) {
	for email, ok := range map[string]bool{"Anu@Example.com ": true, "nope": false, "": false, strings.Repeat("a", 250) + "@x.io": false} {
		r := OTPRequestReq{Email: email}
		herr := r.Validate()
		require.Equal(t, ok, herr == nil, email)
		if ok {
			require.Equal(t, "anu@example.com", r.Email)
		} else {
			require.Equal(t, "a valid email is required", herr.Error())
		}
	}
}

func TestPatchMeValidate(t *testing.T) {
	cases := map[string]string{
		`{}`: "",
		`{"displayName":"Aman K","theme":"mintPeach","timezone":"Asia/Kolkata","avatar":null}`: "",
		`{"displayName":""}`:       "displayName must be 1-40 characters",
		`{"displayName":null}`:     "displayName must be 1-40 characters",
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
