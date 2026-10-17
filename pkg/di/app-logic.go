// Package di wires the application with google/wire. AppLogic is the process-wide accessor the HTTP
// layer uses to reach services, the logger and the configuration.
package di

import (
	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/domain/service"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils/clock"
	"github.com/bubu-bloom/service/pkg/utils/ratelimit"
)

// AppLogic is set once at startup (cmd/server/main.go) or per test.
var AppLogic AppInterface

// AppInterface exposes every dependency the HTTP layer needs.
type AppInterface interface {
	GetLogger() logger.ILogger
	GetConfig() *config.Store
	GetClock() clock.Clock
	GetRateLimiters() *ratelimit.Set
	GetAuthService() service.IAuthService
	GetUserService() service.IUserService
	GetHabitService() service.IHabitService
	GetDailyLogService() service.IDailyLogService
	GetWorkoutService() service.IWorkoutService
	GetSettingsService() service.ISettingsService
	GetSyncService() service.ISyncService
	GetStatsService() service.IStatsService
	GetProvisionService() service.IProvisionService
	GetHealthService() service.IHealthService
}

type appLogic struct {
	logger    logger.ILogger
	config    *config.Store
	clock     clock.Clock
	limiters  *ratelimit.Set
	auth      service.IAuthService
	user      service.IUserService
	habit     service.IHabitService
	dailyLog  service.IDailyLogService
	workout   service.IWorkoutService
	settings  service.ISettingsService
	sync      service.ISyncService
	stats     service.IStatsService
	provision service.IProvisionService
	health    service.IHealthService
}

func (a *appLogic) GetLogger() logger.ILogger                      { return a.logger }
func (a *appLogic) GetConfig() *config.Store                       { return a.config }
func (a *appLogic) GetClock() clock.Clock                          { return a.clock }
func (a *appLogic) GetRateLimiters() *ratelimit.Set                { return a.limiters }
func (a *appLogic) GetAuthService() service.IAuthService           { return a.auth }
func (a *appLogic) GetUserService() service.IUserService           { return a.user }
func (a *appLogic) GetHabitService() service.IHabitService         { return a.habit }
func (a *appLogic) GetDailyLogService() service.IDailyLogService   { return a.dailyLog }
func (a *appLogic) GetWorkoutService() service.IWorkoutService     { return a.workout }
func (a *appLogic) GetSettingsService() service.ISettingsService   { return a.settings }
func (a *appLogic) GetSyncService() service.ISyncService           { return a.sync }
func (a *appLogic) GetStatsService() service.IStatsService         { return a.stats }
func (a *appLogic) GetProvisionService() service.IProvisionService { return a.provision }
func (a *appLogic) GetHealthService() service.IHealthService       { return a.health }
