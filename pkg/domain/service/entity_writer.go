package service

import (
	"context"
	"strings"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/opstatus"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// requireOwner is the service-level viewer guard (defense in depth behind the role middleware).
func requireOwner(a *serviceBos.Actor) apperrors.IHttpError {
	if a == nil || !a.IsOwner() {
		return apperrors.ForbiddenError("read-only access")
	}
	return nil
}

// resultToEntity maps an ApplyResult onto the REST response: the authoritative record on success,
// an HTTP error otherwise.
func resultToEntity(res *serviceBos.ApplyResult) (map[string]any, apperrors.IHttpError) {
	switch res.Status {
	case opstatus.Applied, opstatus.Duplicate:
		return res.Entity.Public(), nil
	case opstatus.Conflict:
		return nil, apperrors.ConflictError("the record was changed elsewhere", map[string]any{"server": res.Entity.Public()})
	default: // rejected
		switch res.Reason {
		case "forbidden":
			return nil, apperrors.ForbiddenError("read-only access")
		case ReasonPeriodTrackerDisabled:
			return nil, apperrors.ForbiddenError("period tracker is disabled")
		case "not_found":
			return nil, apperrors.NotFoundError("not found")
		default:
			return nil, apperrors.BadRequestError(strings.TrimPrefix(res.Reason, "validation_failed: "))
		}
	}
}

func publicList(es []dao.Entity) []map[string]any {
	out := make([]map[string]any, 0, len(es))
	for i := range es {
		out = append(out, es[i].Public())
	}
	return out
}

// entityWriter implements create/patch/delete for one entity type on top of ISyncService.
type entityWriter struct {
	log      logger.ILogger
	typ      string
	entities mongorepo.IEntityRepo
	sync     ISyncService
}

func (w *entityWriter) create(ctx context.Context, a *serviceBos.Actor, req *request.CreateEntityReq) (map[string]any, apperrors.IHttpError) {
	if herr := requireOwner(a); herr != nil {
		return nil, herr
	}
	res, herr := w.sync.Apply(ctx, a, serviceBos.ApplyOp{EntityType: w.typ, EntityID: req.ID, Op: opUpsert, BaseVersion: 0, Payload: req.Payload})
	if herr != nil {
		return nil, herr
	}
	return resultToEntity(res)
}

// find loads the entity so PATCH/DELETE never create records and respond 404 for unknown ids
// (including ids that exist only in another space).
func (w *entityWriter) find(ctx context.Context, a *serviceBos.Actor, id string) (*dao.Entity, apperrors.IHttpError) {
	e, err := w.entities.Get(ctx, a.SpaceID(), w.typ, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.NotFoundError("not found")
		}
		return nil, internalErr(w.log, ctx, "entity lookup failed", err)
	}
	return e, nil
}

func (w *entityWriter) patch(ctx context.Context, a *serviceBos.Actor, req *request.PatchEntityReq) (map[string]any, apperrors.IHttpError) {
	if herr := requireOwner(a); herr != nil {
		return nil, herr
	}
	if _, herr := w.find(ctx, a, req.ID); herr != nil {
		return nil, herr
	}
	if !req.HasVersion {
		return nil, apperrors.BadRequestError("version is required")
	}
	res, herr := w.sync.Apply(ctx, a, serviceBos.ApplyOp{EntityType: w.typ, EntityID: req.ID, Op: opUpsert, BaseVersion: req.Version, Payload: req.Payload})
	if herr != nil {
		return nil, herr
	}
	return resultToEntity(res)
}

// remove deletes with the current version; payload non-nil means a soft deactivation upsert instead of a tombstone.
func (w *entityWriter) remove(ctx context.Context, a *serviceBos.Actor, id string, op string, payload map[string]any) apperrors.IHttpError {
	if herr := requireOwner(a); herr != nil {
		return herr
	}
	e, herr := w.find(ctx, a, id)
	if herr != nil {
		return herr
	}
	res, herr := w.sync.Apply(ctx, a, serviceBos.ApplyOp{EntityType: w.typ, EntityID: id, Op: op, BaseVersion: e.Version, Payload: payload})
	if herr != nil {
		return herr
	}
	if res.Status == opstatus.Applied {
		return nil
	}
	_, herr = resultToEntity(res)
	return herr
}
