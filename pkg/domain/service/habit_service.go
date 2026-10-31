package service

import (
	"context"
	"sort"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// IHabitService manages habits. Deleting soft-deactivates (logs are retained).
type IHabitService interface {
	List(ctx context.Context, actor *serviceBos.Actor, req *request.ListHabitsReq) (*response.ItemsResponse, apperrors.IHttpError)
	Create(ctx context.Context, actor *serviceBos.Actor, req *request.CreateEntityReq) (map[string]any, apperrors.IHttpError)
	Patch(ctx context.Context, actor *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError)
	Delete(ctx context.Context, actor *serviceBos.Actor, id string) apperrors.IHttpError
}

type habitService struct {
	log      logger.ILogger
	entities mongorepo.IEntityRepo
	w        *entityWriter
}

func NewHabitService(log logger.ILogger, entities mongorepo.IEntityRepo, sync ISyncService) IHabitService {
	return &habitService{log: log, entities: entities, w: &entityWriter{log: log, typ: entitytype.Habit, entities: entities, sync: sync}}
}

func (s *habitService) List(ctx context.Context, a *serviceBos.Actor, req *request.ListHabitsReq) (*response.ItemsResponse, apperrors.IHttpError) {
	es, err := s.entities.List(ctx, a.SpaceID(), entitytype.Habit, mongorepo.EntityQuery{})
	if err != nil {
		return nil, internalErr(s.log, ctx, "habit list failed", err)
	}
	out := es[:0]
	for _, e := range es {
		if active, _ := e.Data["isActive"].(bool); active || req.IncludeInactive {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		x, _ := out[i].Data["sortOrder"].(float64)
		y, _ := out[j].Data["sortOrder"].(float64)
		return x < y
	})
	return &response.ItemsResponse{Items: publicList(out)}, nil
}

func (s *habitService) Create(ctx context.Context, a *serviceBos.Actor, req *request.CreateEntityReq) (map[string]any, apperrors.IHttpError) {
	return s.w.create(ctx, a, req)
}

func (s *habitService) Patch(ctx context.Context, a *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError) {
	return s.w.patch(ctx, a, req)
}

func (s *habitService) Delete(ctx context.Context, a *serviceBos.Actor, id string) apperrors.IHttpError {
	return s.w.remove(ctx, a, id, opUpsert, map[string]any{"isActive": false})
}
