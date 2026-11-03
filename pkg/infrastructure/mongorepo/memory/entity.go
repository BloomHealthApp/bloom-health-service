package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type entityRepo struct {
	mu       sync.Mutex
	entities map[string]*dao.Entity
	seq      map[string]int64
}

func NewEntityRepo() mongorepo.IEntityRepo {
	return &entityRepo{entities: map[string]*dao.Entity{}, seq: map[string]int64{}}
}

func ekey(space, typ, id string) string { return space + "|" + typ + "|" + id }

func (r *entityRepo) Get(_ context.Context, space, typ, id string) (*dao.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entities[ekey(space, typ, id)]; ok {
		return e.Clone(), nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *entityRepo) List(_ context.Context, space, typ string, q mongorepo.EntityQuery) ([]dao.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []dao.Entity
	for _, e := range r.entities {
		if e.SpaceID != space || e.Type != typ {
			continue
		}
		if e.DeletedAt != nil && !q.IncludeDeleted {
			continue
		}
		if q.From != "" || q.To != "" {
			d, _ := e.Data["localDate"].(string)
			if d == "" || (q.From != "" && d < q.From) || (q.To != "" && d > q.To) {
				continue
			}
		}
		out = append(out, *e.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt) || (out[i].CreatedAt.Equal(out[j].CreatedAt) && out[i].ID < out[j].ID)
	})
	return out, nil
}

func (r *entityRepo) Save(_ context.Context, e *dao.Entity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := ekey(e.SpaceID, e.Type, e.ID)
	cur, exists := r.entities[k]
	switch {
	case e.Version == 1 && exists:
		return apperrors.ErrRepoConflict
	case e.Version > 1 && (!exists || cur.Version != e.Version-1):
		return apperrors.ErrRepoConflict
	}
	r.seq[e.SpaceID]++
	e.Seq = r.seq[e.SpaceID]
	r.entities[k] = e.Clone()
	return nil
}

func (r *entityRepo) Changes(_ context.Context, space string, after int64, limit int) ([]dao.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []dao.Entity
	for _, e := range r.entities {
		if e.SpaceID == space && e.Seq > after {
			out = append(out, *e.Clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
