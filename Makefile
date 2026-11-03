.PHONY: test test-unit test-integration coverage build build-rest build-grpc-server build-grpc-client build-cron \
	run-rest run-grpc-server run-grpc-client run-cron infra-up infra-down

test: test-unit test-integration

test-unit:
	go test ./...

test-integration:
	go test -tags=integration ./tests/integration/...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

build: build-rest build-grpc-server build-grpc-client build-cron

build-rest:
	mkdir -p bin
	go build -o bin/rest ./cmd/rest

build-grpc-server:
	mkdir -p bin
	go build -o bin/grpc-server ./cmd/grpc-server

build-grpc-client:
	mkdir -p bin
	go build -o bin/grpc-client ./cmd/grpc-client

build-cron:
	mkdir -p bin
	go build -o bin/cron ./cmd/cron

run-rest:
	go run ./cmd/rest

run-grpc-server:
	go run ./cmd/grpc-server

run-grpc-client:
	go run ./cmd/grpc-client

run-cron:
	go run ./cmd/cron

infra-up:
	docker compose up -d --wait

infra-down:
	docker compose down
