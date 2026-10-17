.PHONY: test vet run seed build
test:
	go vet ./... && go test -race ./...
run:
	go run ./cmd/server
seed:
	go run ./cmd/seed
build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server
