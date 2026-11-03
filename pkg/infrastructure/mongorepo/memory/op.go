package memory

import (
	"context"
	"sync"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type opRepo struct {
	mu  sync.Mutex
	ops map[string]*dao.OpRecord
}

func NewOpRepo() mongorepo.IOpRepo { return &opRepo{ops: map[string]*dao.OpRecord{}} }

func (r *opRepo) Get(_ context.Context, id string) (*dao.OpRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.ops[id]; ok {
		return cloneOp(rec), nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *opRepo) Save(_ context.Context, rec *dao.OpRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.ops[rec.OperationID]; ok {
		return apperrors.ErrRepoConflict
	}
	r.ops[rec.OperationID] = cloneOp(rec)
	return nil
}

func cloneOp(rec *dao.OpRecord) *dao.OpRecord {
	c := *rec
	if rec.Entity != nil {
		c.Entity = rec.Entity.Clone()
	}
	return &c
}
