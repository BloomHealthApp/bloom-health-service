package service

import (
	"context"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// IHealthService reports liveness without exposing any database detail.
type IHealthService interface {
	Check(ctx context.Context) (*response.HealthResponse, apperrors.IHttpError)
}

type healthService struct {
	log     logger.ILogger
	pinger  drivers.IPinger
	clock   clock.Clock
	version string
}

// NewHealthService wires the health service.
func NewHealthService(log logger.ILogger, cfg *config.Store, pinger drivers.IPinger, clk clock.Clock) IHealthService {
	return &healthService{log: log, pinger: pinger, clock: clk, version: cfg.Version}
}

func (s *healthService) Check(ctx context.Context) (*response.HealthResponse, apperrors.IHttpError) {
	if err := s.pinger.Ping(ctx); err != nil {
		s.log.Error(ctx, "health: ping failed", map[string]any{"err": err.Error()})
		return nil, apperrors.ServiceUnavailableError("unavailable")
	}
	return &response.HealthResponse{Status: "ok", Time: s.clock.Now().UTC().Format(time.RFC3339), Version: s.version}, nil
}
