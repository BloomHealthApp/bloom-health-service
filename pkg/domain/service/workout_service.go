package service

import (
	"context"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// IWorkoutService manages workout sessions. Deleting writes a tombstone.
type IWorkoutService interface {
	List(ctx context.Context, actor *serviceBos.Actor, req *request.RangeReq) (*response.ItemsResponse, apperrors.IHttpError)
	Create(ctx context.Context, actor *serviceBos.Actor, req *request.CreateEntityReq) (map[string]any, apperrors.IHttpError)
	Patch(ctx context.Context, actor *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError)
	Delete(ctx context.Context, actor *serviceBos.Actor, id string) apperrors.IHttpError
}

type workoutService struct {
	log      logger.ILogger
	entities mongorepo.IEntityRepo
	w        *entityWriter
}

// NewWorkoutService wires the workout service.
func NewWorkoutService(log logger.ILogger, entities mongorepo.IEntityRepo, sync ISyncService) IWorkoutService {
	return &workoutService{log: log, entities: entities, w: &entityWriter{log: log, typ: entitytype.Workout, entities: entities, sync: sync}}
}

func (s *workoutService) List(ctx context.Context, a *serviceBos.Actor, req *request.RangeReq) (*response.ItemsResponse, apperrors.IHttpError) {
	es, err := s.entities.List(ctx, a.SpaceID(), entitytype.Workout, mongorepo.EntityQuery{From: req.From, To: req.To})
	if err != nil {
		return nil, internalErr(s.log, ctx, "workout list failed", err)
	}
	return &response.ItemsResponse{Items: publicList(es)}, nil
}

func (s *workoutService) Create(ctx context.Context, a *serviceBos.Actor, req *request.CreateEntityReq) (map[string]any, apperrors.IHttpError) {
	return s.w.create(ctx, a, req)
}

func (s *workoutService) Patch(ctx context.Context, a *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError) {
	return s.w.patch(ctx, a, req)
}

func (s *workoutService) Delete(ctx context.Context, a *serviceBos.Actor, id string) apperrors.IHttpError {
	return s.w.remove(ctx, a, id, opDelete, nil)
}
