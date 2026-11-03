package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/opstatus"
)

func TestPublicList(t *testing.T) {
	assert.Equal(t, []map[string]any{}, PublicList(nil))
	got := PublicList([]dao.Entity{{ID: "h1", Version: 2, Data: map[string]any{"name": "Walk"}}})
	assert.Len(t, got, 1)
	assert.Equal(t, "h1", got[0]["id"])
	assert.Equal(t, "Walk", got[0]["name"])
}

func TestSyncResult(t *testing.T) {
	e := &dao.Entity{ID: "w1", Type: "workout", Version: 1, CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}
	tests := []struct {
		name     string
		res      bo.ApplyResult
		wantType string
	}{
		{"rejected without entity", bo.ApplyResult{Status: opstatus.Rejected, Reason: "invalid"}, ""},
		{"applied with entity", bo.ApplyResult{Status: opstatus.Applied, Entity: e}, "workout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SyncResult("op1", &tt.res)
			assert.Equal(t, "op1", got.OperationID)
			assert.Equal(t, tt.res.Status, got.Status)
			assert.Equal(t, tt.res.Reason, got.Reason)
			assert.Equal(t, tt.wantType, got.EntityType)
			assert.Equal(t, tt.res.Entity != nil, got.Entity != nil)
		})
	}
}
