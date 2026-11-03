package request

import (
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
