package periodflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValid(t *testing.T) {
	for _, v := range Values() {
		require.True(t, Valid(v), v)
	}
	for _, v := range []string{"", " ", "UNKNOWN", "Unknown"} {
		require.False(t, Valid(v), v)
	}
}
