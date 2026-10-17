package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
	"github.com/bubu-bloom/service/pkg/enums/entitytype"
	"github.com/bubu-bloom/service/pkg/enums/opstatus"
	"github.com/bubu-bloom/service/pkg/enums/userrole"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils/clock"
)

type syncMocks struct {
	entities *mongorepo.MockIEntityRepo
	ops      *mongorepo.MockIOpRepo
	clock    *clock.Fake
}

func createSyncService(ctrl *gomock.Controller, setup func(m *syncMocks)) (ISyncService, *syncMocks) {
	m := &syncMocks{entities: mongorepo.NewMockIEntityRepo(ctrl), ops: mongorepo.NewMockIOpRepo(ctrl), clock: clock.NewFake()}
	if setup != nil {
		setup(m)
	}
	return NewSyncService(logger.NewDiscard(), m.entities, m.ops, m.clock), m
}

func actorWithRole(role userrole.Role) *serviceBos.Actor {
	return &serviceBos.Actor{
		User:       &dao.User{ID: "u1", Timezone: "UTC"},
		Membership: &dao.Membership{ID: "m1", SpaceID: "space1", UserID: "u1", Role: role},
	}
}

func habitEntity(version int, updated time.Time) *dao.Entity {
	return &dao.Entity{ID: "h1", Type: entitytype.Habit, SpaceID: "space1", Version: version, CreatedAt: updated, UpdatedAt: updated,
		Data: map[string]any{"title": "Read", "trackingType": "boolean", "category": "custom", "targetComparison": "atLeast",
			"target": float64(1), "targetMax": nil, "unit": nil, "icon": "x", "schedule": map[string]any{"days": []any{}},
			"reminder": nil, "isActive": true, "sortOrder": float64(0), "isDefault": false}}
}

func TestApplyViewerGuardTouchesNothing(t *testing.T) {
	ctrl := gomock.NewController(t) // no expectations: any repo call fails the test
	svc, _ := createSyncService(ctrl, nil)
	res, herr := svc.Apply(context.Background(), actorWithRole(userrole.Viewer), serviceBos.ApplyOp{
		OperationID: "op1", EntityType: entitytype.Habit, EntityID: "h1", Op: "delete", BaseVersion: 1})
	require.Nil(t, herr)
	require.Equal(t, opstatus.Rejected, res.Status)
	require.Equal(t, "forbidden", res.Reason)

	_, herr = svc.Push(context.Background(), actorWithRole(userrole.Viewer), &request.SyncPushReq{Operations: []request.PushOp{{OperationID: "op1"}}})
	require.NotNil(t, herr)
	require.Equal(t, apperrors.CodeForbidden, herr.GetCode())
}

func TestApplyIdempotency(t *testing.T) {
	ctx := context.Background()
	prior := &dao.OpRecord{OperationID: "op1", SpaceID: "space1", Status: opstatus.Applied, Entity: habitEntity(1, time.Now())}

	t.Run("replay of an applied operation is a duplicate and writes nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createSyncService(ctrl, func(m *syncMocks) { m.ops.EXPECT().Get(gomock.Any(), "op1").Return(prior, nil) })
		res, herr := svc.Apply(ctx, actorWithRole(userrole.Owner), serviceBos.ApplyOp{OperationID: "op1", EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert"})
		require.Nil(t, herr)
		require.Equal(t, opstatus.Duplicate, res.Status)
		require.Equal(t, 1, res.Entity.Version)
	})
	t.Run("replayed conflict stays a conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		rec := &dao.OpRecord{OperationID: "op1", SpaceID: "space1", Status: opstatus.Conflict, Entity: habitEntity(2, time.Now())}
		svc, _ := createSyncService(ctrl, func(m *syncMocks) { m.ops.EXPECT().Get(gomock.Any(), "op1").Return(rec, nil) })
		res, _ := svc.Apply(ctx, actorWithRole(userrole.Owner), serviceBos.ApplyOp{OperationID: "op1", EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert"})
		require.Equal(t, opstatus.Conflict, res.Status)
	})
	t.Run("operation id from another space is rejected without leaking the record", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		other := *prior
		other.SpaceID = "space2"
		svc, _ := createSyncService(ctrl, func(m *syncMocks) { m.ops.EXPECT().Get(gomock.Any(), "op1").Return(&other, nil) })
		res, _ := svc.Apply(ctx, actorWithRole(userrole.Owner), serviceBos.ApplyOp{OperationID: "op1", EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert"})
		require.Equal(t, opstatus.Rejected, res.Status)
		require.Nil(t, res.Entity)
	})
	t.Run("lost race on the op record returns the stored outcome", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createSyncService(ctrl, func(m *syncMocks) {
			gomock.InOrder(
				m.ops.EXPECT().Get(gomock.Any(), "op1").Return(nil, apperrors.ErrRepoNotFound),
				m.ops.EXPECT().Get(gomock.Any(), "op1").Return(prior, nil),
			)
			m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(nil, apperrors.ErrRepoNotFound)
			m.entities.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			m.ops.EXPECT().Save(gomock.Any(), gomock.Any()).Return(apperrors.ErrRepoConflict)
		})
		res, herr := svc.Apply(ctx, actorWithRole(userrole.Owner), serviceBos.ApplyOp{OperationID: "op1", EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert",
			Payload: map[string]any{"title": "Read", "trackingType": "boolean"}})
		require.Nil(t, herr)
		require.Equal(t, opstatus.Duplicate, res.Status)
	})
}

func TestApplyWrites(t *testing.T) {
	ctx := context.Background()
	owner := actorWithRole(userrole.Owner)
	create := serviceBos.ApplyOp{EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert", BaseVersion: 0,
		Payload: map[string]any{"title": " Read ", "trackingType": "boolean", "isDefault": true}}

	t.Run("create assigns version 1, trims input and protects server-only fields", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var saved *dao.Entity
		svc, _ := createSyncService(ctrl, func(m *syncMocks) {
			m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(nil, apperrors.ErrRepoNotFound)
			m.entities.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *dao.Entity) error {
				saved = e
				return nil
			})
		})
		res, herr := svc.Apply(ctx, owner, create)
		require.Nil(t, herr)
		require.Equal(t, opstatus.Applied, res.Status)
		require.Equal(t, 1, saved.Version)
		require.Equal(t, "Read", saved.Data["title"])
		require.Equal(t, false, saved.Data["isDefault"])
		require.Equal(t, "space1", saved.SpaceID)
	})
	t.Run("stale base version is a conflict carrying the server record", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createSyncService(ctrl, func(m *syncMocks) {
			m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(habitEntity(3, time.Now()), nil)
		})
		res, herr := svc.Apply(ctx, owner, serviceBos.ApplyOp{EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert", BaseVersion: 1, Payload: map[string]any{"target": 5.0}})
		require.Nil(t, herr)
		require.Equal(t, opstatus.Conflict, res.Status)
		require.Equal(t, 3, res.Entity.Version)
	})
	t.Run("compare-and-swap failure at save time becomes a conflict with the current record", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createSyncService(ctrl, func(m *syncMocks) {
			gomock.InOrder(
				m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(habitEntity(1, time.Now()), nil),
				m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(habitEntity(2, time.Now()), nil),
			)
			m.entities.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *dao.Entity) error {
				require.Equal(t, 2, e.Version, "writes version+1 and relies on CAS against version-1")
				return apperrors.ErrRepoConflict
			})
		})
		res, herr := svc.Apply(ctx, owner, serviceBos.ApplyOp{EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert", BaseVersion: 1, Payload: map[string]any{"target": 5.0}})
		require.Nil(t, herr)
		require.Equal(t, opstatus.Conflict, res.Status)
		require.Equal(t, 2, res.Entity.Version)
	})
	t.Run("mass-assignment of server fields is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createSyncService(ctrl, func(m *syncMocks) {
			m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(nil, apperrors.ErrRepoNotFound)
		})
		res, _ := svc.Apply(ctx, owner, serviceBos.ApplyOp{EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert",
			Payload: map[string]any{"title": "x", "trackingType": "boolean", "sharedSpaceId": "evil"}})
		require.Equal(t, opstatus.Rejected, res.Status)
		require.Contains(t, res.Reason, "validation_failed")
	})
	t.Run("repository failure is an internal error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createSyncService(ctrl, func(m *syncMocks) {
			m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(nil, errors.New("boom"))
		})
		res, herr := svc.Apply(ctx, owner, create)
		require.Nil(t, res)
		require.NotNil(t, herr)
		require.Equal(t, apperrors.CodeInternal, herr.GetCode())
		require.NotContains(t, herr.Error(), "boom")
	})
	t.Run("daily log last-writer-wins when the client timestamp is newer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		today := time.Now().UTC().Format("2006-01-02")
		server := &dao.Entity{ID: "h1:" + today, Type: entitytype.DailyLog, SpaceID: "space1", Version: 2, UpdatedAt: time.Now().Add(-time.Hour),
			CreatedAt: time.Now().Add(-time.Hour), Data: map[string]any{"habitId": "h1", "localDate": today, "value": 1.0, "status": "complete", "note": nil}}
		svc, _ := createSyncService(ctrl, func(m *syncMocks) {
			m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(habitEntity(1, time.Now()), nil)
			m.entities.EXPECT().Get(gomock.Any(), "space1", entitytype.DailyLog, "h1:"+today).Return(server, nil)
			m.entities.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		})
		ts := time.Now().Add(time.Minute)
		res, herr := svc.Apply(ctx, owner, serviceBos.ApplyOp{EntityType: entitytype.DailyLog, EntityID: "h1:" + today, Op: "upsert",
			BaseVersion: 1, Payload: map[string]any{"value": 0.0}, ClientTimestamp: &ts})
		require.Nil(t, herr)
		require.Equal(t, opstatus.Applied, res.Status)
		require.Equal(t, "lww", res.Resolved)
		require.Equal(t, 3, res.Entity.Version)
		require.Equal(t, "incomplete", res.Entity.Data["status"], "status recomputed from the habit target")
	})
}

func TestPushRecordsOutcomePerOperation(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _ := createSyncService(ctrl, func(m *syncMocks) {
		m.ops.EXPECT().Get(gomock.Any(), "dup").Return(&dao.OpRecord{OperationID: "dup", SpaceID: "space1", Status: opstatus.Applied, Entity: habitEntity(1, time.Now())}, nil)
	})
	resp, herr := svc.Push(context.Background(), actorWithRole(userrole.Owner), &request.SyncPushReq{Operations: []request.PushOp{
		{OperationID: "", EntityType: entitytype.Habit},
		{OperationID: "dup", EntityType: entitytype.Habit, EntityID: "h1", Op: "upsert"},
	}})
	require.Nil(t, herr)
	require.Len(t, resp.Results, 2)
	require.Equal(t, "invalid_operation_id", resp.Results[0].Reason)
	require.Equal(t, opstatus.Duplicate, resp.Results[1].Status)
	require.Equal(t, entitytype.Habit, resp.Results[1].EntityType)
}

func TestPullPaginatesByCursor(t *testing.T) {
	ctrl := gomock.NewController(t)
	rows := []dao.Entity{*habitEntity(1, time.Now()), *habitEntity(1, time.Now()), *habitEntity(1, time.Now())}
	for i := range rows {
		rows[i].Seq = int64(i + 1)
	}
	svc, _ := createSyncService(ctrl, func(m *syncMocks) {
		m.entities.EXPECT().Changes(gomock.Any(), "space1", int64(0), 3).Return(rows, nil) // limit+1 rows requested
	})
	resp, herr := svc.Pull(context.Background(), actorWithRole(userrole.Viewer), &request.SyncPullReq{Limit: 2})
	require.Nil(t, herr)
	require.True(t, resp.HasMore)
	require.Len(t, resp.Changes["habits"], 2)
	require.Equal(t, "czoy", resp.Cursor) // base64url("s:2")
}
