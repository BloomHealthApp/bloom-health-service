package request

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
