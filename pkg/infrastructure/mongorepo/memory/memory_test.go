package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo/memory"
)

func ent(id string, version int, date string) *dao.Entity {
	now := time.Now()
	return &dao.Entity{ID: id, Type: "dailyLog", SpaceID: "s1", Version: version, CreatedAt: now, UpdatedAt: now, Data: map[string]any{"localDate": date}}
}

func TestEntityRepoCompareAndSwapAndSeq(t *testing.T) {
	ctx := context.Background()
	r := memory.NewEntityRepo()
	a := ent("a", 1, "2026-01-01")
	require.NoError(t, r.Save(ctx, a))
	require.EqualValues(t, 1, a.Seq)
	require.ErrorIs(t, r.Save(ctx, ent("a", 1, "2026-01-01")), apperrors.ErrRepoConflict, "duplicate insert")
	require.ErrorIs(t, r.Save(ctx, ent("a", 3, "2026-01-01")), apperrors.ErrRepoConflict, "version gap")
	require.ErrorIs(t, r.Save(ctx, ent("zz", 2, "2026-01-01")), apperrors.ErrRepoConflict, "update of missing record")
	a2 := ent("a", 2, "2026-01-01")
	require.NoError(t, r.Save(ctx, a2))
	require.EqualValues(t, 2, a2.Seq, "every accepted write gets the next per-space seq")

	changes, err := r.Changes(ctx, "s1", 1, 10)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	_, err = r.Get(ctx, "other-space", "dailyLog", "a")
	require.ErrorIs(t, err, apperrors.ErrRepoNotFound, "spaces are isolated")
}

func TestEntityRepoListFiltersByDateAndTombstones(t *testing.T) {
	ctx := context.Background()
	r := memory.NewEntityRepo()
	require.NoError(t, r.Save(ctx, ent("a", 1, "2026-01-01")))
	require.NoError(t, r.Save(ctx, ent("b", 1, "2026-02-01")))
	now := time.Now()
	dead := ent("c", 1, "2026-01-15")
	dead.DeletedAt = &now
	require.NoError(t, r.Save(ctx, dead))

	got, _ := r.List(ctx, "s1", "dailyLog", mongorepo.EntityQuery{From: "2026-01-01", To: "2026-01-31"})
	require.Len(t, got, 1)
	got, _ = r.List(ctx, "s1", "dailyLog", mongorepo.EntityQuery{From: "2026-01-01", To: "2026-01-31", IncludeDeleted: true})
	require.Len(t, got, 2)
}

func TestSessionRepoMarkUsedIsAtomicOnce(t *testing.T) {
	ctx := context.Background()
	r := memory.NewSessionRepo()
	require.NoError(t, r.Save(ctx, &dao.RefreshSession{ID: "s", FamilyID: "f", TokenHash: "h"}))
	ok, _ := r.MarkUsed(ctx, "s")
	require.True(t, ok)
	ok, _ = r.MarkUsed(ctx, "s")
	require.False(t, ok)
	require.NoError(t, r.RevokeFamily(ctx, "f"))
	s, _ := r.ByHash(ctx, "h")
	require.NotNil(t, s.RevokedAt)
}

func TestOpRepoUniqueOperationID(t *testing.T) {
	ctx := context.Background()
	r := memory.NewOpRepo()
	require.NoError(t, r.Save(ctx, &dao.OpRecord{OperationID: "op", SpaceID: "s1", Status: "applied"}))
	require.ErrorIs(t, r.Save(ctx, &dao.OpRecord{OperationID: "op", SpaceID: "s1"}), apperrors.ErrRepoConflict)
}
