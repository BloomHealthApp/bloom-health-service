package service

import (
	"context"

	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/dto/response"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
	"github.com/bubu-bloom/service/pkg/enums/entitytype"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/logger"
)

// IDailyLogService reads and writes per-habit daily logs (unique per habit and local date).
type IDailyLogService interface {
	// List returns logs in the range including tombstones.
	List(ctx context.Context, actor *serviceBos.Actor, req *request.RangeReq) (*response.ItemsResponse, apperrors.IHttpError)
	Put(ctx context.Context, actor *serviceBos.Actor, req *request.PutLogReq) (map[string]any, apperrors.IHttpError)
}

type dailyLogService struct {
	log      logger.ILogger
	entities mongorepo.IEntityRepo
	sync     ISyncService
}

// NewDailyLogService wires the daily log service.
func NewDailyLogService(log logger.ILogger, entities mongorepo.IEntityRepo, sync ISyncService) IDailyLogService {
	return &dailyLogService{log: log, entities: entities, sync: sync}
}

func (s *dailyLogService) List(ctx context.Context, a *serviceBos.Actor, req *request.RangeReq) (*response.ItemsResponse, apperrors.IHttpError) {
	es, err := s.entities.List(ctx, a.SpaceID(), entitytype.DailyLog, mongorepo.EntityQuery{From: req.From, To: req.To, IncludeDeleted: true})
	if err != nil {
		return nil, internalErr(s.log, ctx, "log list failed", err)
	}
	return &response.ItemsResponse{Items: publicList(es)}, nil
}

func (s *dailyLogService) Put(ctx context.Context, a *serviceBos.Actor, req *request.PutLogReq) (map[string]any, apperrors.IHttpError) {
	if herr := requireOwner(a); herr != nil {
		return nil, herr
	}
	payload := map[string]any{"value": nil, "note": nil}
	if req.Value != nil {
		payload["value"] = *req.Value
	}
	if req.Note != nil {
		payload["note"] = *req.Note
	}
	if req.Status != "" {
		payload["status"] = req.Status
	}
	res, herr := s.sync.Apply(ctx, a, serviceBos.ApplyOp{EntityType: entitytype.DailyLog, EntityID: LogID(req.HabitID, req.LocalDate),
		Op: opUpsert, BaseVersion: req.BaseVersion, Payload: payload})
	if herr != nil {
		return nil, herr
	}
	return resultToEntity(res)
}
