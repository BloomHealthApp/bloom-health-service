package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name, in, cc, want string
		bad                bool
	}{
		{name: "bare 10 digits gets default +91", in: "9876543210", want: "+919876543210"},
		{name: "spaces and dashes stripped", in: " 98765-43210 ", want: "+919876543210"},
		{name: "parentheses stripped", in: "(987) 654 3210", want: "+919876543210"},
		{name: "plus prefixed kept", in: "+1 (415) 555-2671", want: "+14155552671"},
		{name: "plus prefixed india", in: "+91 98765 43210", want: "+919876543210"},
		{name: "configured country code", in: "4155552671", cc: "+1", want: "+14155552671"},
		{name: "minimum 8 digits", in: "+12345678", want: "+12345678"},
		{name: "maximum 15 digits", in: "+123456789012345", want: "+123456789012345"},
		{name: "too short", in: "+1234567", bad: true},
		{name: "too long", in: "+1234567890123456", bad: true},
		{name: "bare 9 digits", in: "987654321", bad: true},
		{name: "bare 12 digits without plus", in: "919876543210", bad: true},
		{name: "letters", in: "98765abcde", bad: true},
		{name: "empty", in: "  ", bad: true},
		{name: "only plus", in: "+", bad: true},
		{name: "leading zero country", in: "+0123456789", bad: true},
		{name: "plus in the middle", in: "98+7654321", bad: true},
		{name: "bad configured code", in: "9876543210", cc: "91", bad: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePhone(tc.in, tc.cc)
			if tc.bad {
				require.ErrorIs(t, err, ErrInvalidPhone)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
