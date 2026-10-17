package service

import (
	"context"
	"sort"

	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/dto/response"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
	"github.com/bubu-bloom/service/pkg/enums/entitytype"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/logger"
)

// IPeriodService manages the private period tracker. Every method is owner-only and additionally
// requires the owner's effective periodTrackerEnabled; viewers and disabled trackers get 403.
// Deleting writes a tombstone.
type IPeriodService interface {
	List(ctx context.Context, actor *serviceBos.Actor, req *request.RangeReq) (*response.ItemsResponse, apperrors.IHttpError)
	Create(ctx context.Context, actor *serviceBos.Actor, req *request.CreateEntityReq) (map[string]any, apperrors.IHttpError)
	Patch(ctx context.Context, actor *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError)
	Delete(ctx context.Context, actor *serviceBos.Actor, id string) apperrors.IHttpError
}

type periodService struct {
	log      logger.ILogger
	entities mongorepo.IEntityRepo
	w        *entityWriter
}

// NewPeriodService wires the period service.
func NewPeriodService(log logger.ILogger, entities mongorepo.IEntityRepo, sync ISyncService) IPeriodService {
	return &periodService{log: log, entities: entities, w: &entityWriter{log: log, typ: entitytype.Period, entities: entities, sync: sync}}
}

// guard is the privacy gate: owner only, tracker enabled.
func (s *periodService) guard(a *serviceBos.Actor) apperrors.IHttpError {
	if herr := requireOwner(a); herr != nil {
		return herr
	}
	if !a.User.IsPeriodTrackerEnabled() {
		return apperrors.ForbiddenError("period tracker is disabled")
	}
	return nil
}

func (s *periodService) List(ctx context.Context, a *serviceBos.Actor, req *request.RangeReq) (*response.ItemsResponse, apperrors.IHttpError) {
	if herr := s.guard(a); herr != nil {
		return nil, herr
	}
	es, err := s.entities.List(ctx, a.SpaceID(), entitytype.Period, mongorepo.EntityQuery{})
	if err != nil {
		return nil, internalErr(s.log, ctx, "period list failed", err)
	}
	items := es[:0]
	for _, e := range es {
		// optional overlap filter on [from,to]; an ongoing period has no end
		start, _ := e.Data["startDate"].(string)
		end, _ := e.Data["endDate"].(string)
		if req.To != "" && start > req.To {
			continue
		}
		if req.From != "" && end != "" && end < req.From {
			continue
		}
		items = append(items, e)
	}
	sortByStart(items)
	return &response.ItemsResponse{Items: publicList(items)}, nil
}

func (s *periodService) Create(ctx context.Context, a *serviceBos.Actor, req *request.CreateEntityReq) (map[string]any, apperrors.IHttpError) {
	if herr := s.guard(a); herr != nil {
		return nil, herr
	}
	return s.w.create(ctx, a, req)
}

func (s *periodService) Patch(ctx context.Context, a *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError) {
	if herr := s.guard(a); herr != nil {
		return nil, herr
	}
	return s.w.patch(ctx, a, req)
}

func (s *periodService) Delete(ctx context.Context, a *serviceBos.Actor, id string) apperrors.IHttpError {
	if herr := s.guard(a); herr != nil {
		return herr
	}
	return s.w.remove(ctx, a, id, opDelete, nil)
}

func sortByStart(es []dao.Entity) {
	sort.SliceStable(es, func(i, j int) bool {
		a, _ := es[i].Data["startDate"].(string)
		b, _ := es[j].Data["startDate"].(string)
		return a < b
	})
}
