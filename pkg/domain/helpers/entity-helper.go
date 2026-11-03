package helpers

import (
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
)

// PublicList renders entities as their API representation; never nil.
func PublicList(es []dao.Entity) []map[string]any {
	out := make([]map[string]any, 0, len(es))
	for i := range es {
		out = append(out, es[i].Public())
	}
	return out
}

// SyncResult renders the outcome of one pushed operation.
func SyncResult(operationID string, res *bo.ApplyResult) response.SyncResult {
	item := response.SyncResult{OperationID: operationID, Status: res.Status, Reason: res.Reason, Resolved: res.Resolved}
	if res.Entity != nil {
		item.Entity = res.Entity.Public()
		item.EntityType = res.Entity.Type
	}
	return item
}
