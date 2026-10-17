MODULE      := github.com/bubu-bloom/service
BIN_DIR     := bin
GOBIN       := $(shell go env GOPATH)/bin
MOCKGEN     := $(GOBIN)/mockgen
WIRE        := $(GOBIN)/wire

SERVICE_IFACES   := IAuthService,IUserService,IHabitService,IDailyLogService,IWorkoutService,ISettingsService,ISyncService,IStatsService,IProvisionService,IHealthService
MONGOREPO_IFACES := IUserRepo,ISpaceRepo,IMembershipRepo,IEntityRepo,IOpRepo,IOTPRepo,ISessionRepo
DRIVER_IFACES    := IPinger,IMongoDB

.PHONY: all build run seed wire install-tools mock-all mock-service mock-mongorepo mock-drivers mock-smtp vet test test-coverage tidy clean

all: wire mock-all vet test build

install-tools:
	@test -x $(MOCKGEN) || go install github.com/golang/mock/mockgen@v1.6.0
	@test -x $(WIRE) || go install github.com/google/wire/cmd/wire@latest

build:
	CGO_ENABLED=0 go build -trimpath -o $(BIN_DIR)/http-server ./cmd
	CGO_ENABLED=0 go build -trimpath -o $(BIN_DIR)/seed ./cmd/seed

run:
	go run ./cmd

seed:
	go run ./cmd/seed

# Regenerates pkg/di/wire_gen.go from pkg/di/container.go.
wire: install-tools
	cd pkg/di && $(WIRE)

mock-all: mock-service mock-mongorepo mock-drivers mock-smtp

mock-service: install-tools
	$(MOCKGEN) -destination=pkg/domain/service/mock_gen.go -package=service $(MODULE)/pkg/domain/service $(SERVICE_IFACES)

mock-mongorepo: install-tools
	$(MOCKGEN) -destination=pkg/infrastructure/mongorepo/mock_gen.go -package=mongorepo $(MODULE)/pkg/infrastructure/mongorepo $(MONGOREPO_IFACES)

mock-drivers: install-tools
	$(MOCKGEN) -destination=pkg/drivers/mock_gen.go -package=drivers $(MODULE)/pkg/drivers $(DRIVER_IFACES)

mock-smtp: install-tools
	$(MOCKGEN) -destination=pkg/infrastructure/transport/smtp/mock_gen.go -package=smtp $(MODULE)/pkg/infrastructure/transport/smtp IMailer

vet:
	go vet ./...

test:
	go vet ./... && go test -race -count=1 ./...

# Only packages that have tests are run with coverage (newer toolchains need `covdata` for test-less packages).
TESTED_PKGS = $(shell go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...)

test-coverage:
	go test -race -count=1 -coverpkg=./pkg/...,./config/... -coverprofile=coverage.out -covermode=atomic $(TESTED_PKGS)
	go tool cover -func=coverage.out | tail -n 1

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR) coverage.out
