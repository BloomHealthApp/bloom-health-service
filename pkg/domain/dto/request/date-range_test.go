package request

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRangeValidate(t *testing.T) {
	bad := []RangeReq{{Required: true}, {From: "2026-1-1"}, {To: "x"}, {From: "2026-02-01", To: "2026-01-01"}, {From: "2020-01-01", To: "2026-12-31"}}
	for _, r := range bad {
		require.NotNil(t, r.Validate(), r)
	}
	ok := RangeReq{From: "2026-01-01", To: "2026-01-31", Required: true}
	require.Nil(t, ok.Validate())
}
