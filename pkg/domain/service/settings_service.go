package service

import (
	"context"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/validation"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// ISettingsService reads (both roles) and updates (owner) the shared space settings.
type ISettingsService interface {
	Get(ctx context.Context, actor *serviceBos.Actor) (map[string]any, apperrors.IHttpError)
	Patch(ctx context.Context, actor *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError)
}

type settingsService struct {
	log      logger.ILogger
	entities mongorepo.IEntityRepo
	sync     ISyncService
	clock    clock.Clock
}

func NewSettingsService(log logger.ILogger, entities mongorepo.IEntityRepo, sync ISyncService, clk clock.Clock) ISettingsService {
	return &settingsService{log: log, entities: entities, sync: sync, clock: clk}
}

func (s *settingsService) Get(ctx context.Context, a *serviceBos.Actor) (map[string]any, apperrors.IHttpError) {
	e, err := s.entities.Get(ctx, a.SpaceID(), entitytype.Settings, constants.SettingsID)
	if apperrors.IsNotFound(err) {
		data, _ := validation.Merge(entitytype.Settings, nil, validation.DefaultSettings(), validation.Context{})
		now := s.clock.Now().UTC()
		return (&dao.Entity{ID: constants.SettingsID, Data: data, CreatedAt: now, UpdatedAt: now}).Public(), nil
	}
	if err != nil {
		return nil, internalErr(s.log, ctx, "settings lookup failed", err)
	}
	pub := e.Public()
	if _, ok := pub["sundayRest"]; !ok {
		pub["sundayRest"] = true // legacy settings without the field: Sunday rest is on by default
	}
	return pub, nil
}

// Patch updates settings; a missing version means "no known base" (0), which conflicts with an existing record.
func (s *settingsService) Patch(ctx context.Context, a *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError) {
	if herr := requireOwner(a); herr != nil {
		return nil, herr
	}
	res, herr := s.sync.Apply(ctx, a, serviceBos.ApplyOp{EntityType: entitytype.Settings, EntityID: constants.SettingsID,
		Op: opUpsert, BaseVersion: req.Version, Payload: req.Payload})
	if herr != nil {
		return nil, herr
	}
	return resultToEntity(res)
}
