package request

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
