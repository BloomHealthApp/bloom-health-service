package service

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/opstatus"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// Viewers are read-only: every write method must return 403 before touching sync or the repositories.
func TestViewerGuardOnWriteServices(t *testing.T) {
	ctx := context.Background()
	viewer := actorWithRole(userrole.Viewer)
	ctrl := gomock.NewController(t) // strict mocks without expectations
	entities := mongorepo.NewMockIEntityRepo(ctrl)
	sync := NewMockISyncService(ctrl)
	habits := NewHabitService(logger.NewDiscard(), entities, sync)
	workouts := NewWorkoutService(logger.NewDiscard(), entities, sync)
	logs := NewDailyLogService(logger.NewDiscard(), entities, sync)
	settings := NewSettingsService(logger.NewDiscard(), entities, sync, nil)

	calls := map[string]func() apperrors.IHttpError{
		"habit create": func() apperrors.IHttpError {
			_, e := habits.Create(ctx, viewer, &request.CreateEntityReq{ID: "h", Payload: map[string]any{}})
			return e
		},
		"habit patch": func() apperrors.IHttpError {
			_, e := habits.Patch(ctx, viewer, &request.PatchEntityReq{ID: "h", Version: 1, HasVersion: true})
			return e
		},
		"habit delete": func() apperrors.IHttpError { return habits.Delete(ctx, viewer, "h") },
		"workout create": func() apperrors.IHttpError {
			_, e := workouts.Create(ctx, viewer, &request.CreateEntityReq{})
			return e
		},
		"workout patch": func() apperrors.IHttpError {
			_, e := workouts.Patch(ctx, viewer, &request.PatchEntityReq{ID: "w"})
			return e
		},
		"workout delete": func() apperrors.IHttpError { return workouts.Delete(ctx, viewer, "w") },
		"log put": func() apperrors.IHttpError {
			_, e := logs.Put(ctx, viewer, &request.PutLogReq{HabitID: "h", LocalDate: "2026-01-01"})
			return e
		},
		"settings patch": func() apperrors.IHttpError { _, e := settings.Patch(ctx, viewer, &request.PatchEntityReq{}); return e },
	}
	for name, call := range calls {
		herr := call()
		require.NotNil(t, herr, name)
		status, _ := herr.Get()
		require.Equal(t, 403, status, name)
	}
}

func TestOwnerWritesDelegateToSyncAndMapOutcomes(t *testing.T) {
	ctx := context.Background()
	owner := actorWithRole(userrole.Owner)

	t.Run("create maps applied to the public record", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		sync := NewMockISyncService(ctrl)
		sync.EXPECT().Apply(gomock.Any(), owner, gomock.Any()).DoAndReturn(func(_ context.Context, _ *serviceBos.Actor, op serviceBos.ApplyOp) (*serviceBos.ApplyResult, apperrors.IHttpError) {
			require.Equal(t, "h-new", op.EntityID)
			require.Equal(t, 0, op.BaseVersion)
			return &serviceBos.ApplyResult{Status: opstatus.Applied, Entity: habitEntity(1, time.Now())}, nil
		})
		habits := NewHabitService(logger.NewDiscard(), mongorepo.NewMockIEntityRepo(ctrl), sync)
		out, herr := habits.Create(ctx, owner, &request.CreateEntityReq{ID: "h-new", Payload: map[string]any{"title": "x"}})
		require.Nil(t, herr)
		require.EqualValues(t, 1, out["version"])
	})
	t.Run("patch of an unknown id is 404 and never reaches sync", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		entities := mongorepo.NewMockIEntityRepo(ctrl)
		entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "nope").Return(nil, apperrors.ErrRepoNotFound)
		habits := NewHabitService(logger.NewDiscard(), entities, NewMockISyncService(ctrl))
		_, herr := habits.Patch(ctx, owner, &request.PatchEntityReq{ID: "nope", Version: 1, HasVersion: true})
		require.NotNil(t, herr)
		require.Equal(t, apperrors.CodeNotFound, herr.GetCode())
	})
	t.Run("patch without a version is 400 once the record exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		entities := mongorepo.NewMockIEntityRepo(ctrl)
		entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(habitEntity(1, time.Now()), nil)
		habits := NewHabitService(logger.NewDiscard(), entities, NewMockISyncService(ctrl))
		_, herr := habits.Patch(ctx, owner, &request.PatchEntityReq{ID: "h1"})
		require.NotNil(t, herr)
		require.Equal(t, apperrors.CodeValidationFailed, herr.GetCode())
	})
	t.Run("stale patch is 409 with the server record in details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		entities := mongorepo.NewMockIEntityRepo(ctrl)
		entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(habitEntity(3, time.Now()), nil)
		sync := NewMockISyncService(ctrl)
		sync.EXPECT().Apply(gomock.Any(), owner, gomock.Any()).Return(&serviceBos.ApplyResult{Status: opstatus.Conflict, Entity: habitEntity(3, time.Now())}, nil)
		habits := NewHabitService(logger.NewDiscard(), entities, sync)
		_, herr := habits.Patch(ctx, owner, &request.PatchEntityReq{ID: "h1", Version: 1, HasVersion: true})
		require.NotNil(t, herr)
		status, body := herr.Get()
		require.Equal(t, 409, status)
		server := body.(apperrors.ErrorBody).Error.Details["server"].(map[string]any)
		require.EqualValues(t, 3, server["version"])
	})
	t.Run("habit delete deactivates instead of tombstoning", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		entities := mongorepo.NewMockIEntityRepo(ctrl)
		entities.EXPECT().Get(gomock.Any(), "space1", entitytype.Habit, "h1").Return(habitEntity(4, time.Now()), nil)
		sync := NewMockISyncService(ctrl)
		sync.EXPECT().Apply(gomock.Any(), owner, gomock.Any()).DoAndReturn(func(_ context.Context, _ *serviceBos.Actor, op serviceBos.ApplyOp) (*serviceBos.ApplyResult, apperrors.IHttpError) {
			require.Equal(t, "upsert", op.Op)
			require.Equal(t, 4, op.BaseVersion)
			require.Equal(t, map[string]any{"isActive": false}, op.Payload)
			return &serviceBos.ApplyResult{Status: opstatus.Applied, Entity: &dao.Entity{}}, nil
		})
		require.Nil(t, NewHabitService(logger.NewDiscard(), entities, sync).Delete(ctx, owner, "h1"))
	})
}
