//go:generate go run github.com/google/wire/cmd/wire
//go:build wireinject
// +build wireinject

package di

import (
	"github.com/google/wire"

	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/domain/service"
	"github.com/bubu-bloom/service/pkg/infrastructure/transport/smtp"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils/clock"
	"github.com/bubu-bloom/service/pkg/utils/ratelimit"
)

// services are shared by the production and the test injector.
var services = wire.NewSet(
	service.NewAuthService,
	service.NewUserService,
	service.NewHabitService,
	service.NewDailyLogService,
	service.NewWorkoutService,
	service.NewPeriodService,
	service.NewSettingsService,
	service.NewSyncService,
	service.NewStatsService,
	service.NewProvisionService,
	service.NewHealthService,
)

var repositoryFields = wire.FieldsOf(new(*Repositories),
	"Users", "Spaces", "Memberships", "Entities", "Ops", "OTPs", "Sessions", "Pinger")

var app = wire.NewSet(
	ratelimit.NewSet,
	services,
	repositoryFields,
	wire.Struct(new(appLogic), "*"),
	wire.Bind(new(AppInterface), new(*appLogic)),
)

// InitializeApp builds the production object graph from the environment.
func InitializeApp() (AppInterface, func(), error) {
	wire.Build(
		config.NewConfig,
		logger.NewLogger,
		clock.NewRealClock,
		smtp.NewMailer,
		NewRepositories,
		app,
	)
	return nil, nil, nil
}

// InitializeTestApp builds the graph from explicit dependencies (in-memory repositories, fake clock,
// capturing mailer) so tests exercise the real services and HTTP layer.
func InitializeTestApp(cfg *config.Store, log logger.ILogger, clk clock.Clock, mailer smtp.IMailer, repos *Repositories) (AppInterface, error) {
	wire.Build(app)
	return nil, nil
}
