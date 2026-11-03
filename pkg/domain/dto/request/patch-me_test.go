package request

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
