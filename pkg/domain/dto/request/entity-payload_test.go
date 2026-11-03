package request

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
