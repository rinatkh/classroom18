.DEFAULT_GOAL := help
GO ?= go
DOCKER ?= docker
COMPOSE = $(DOCKER) compose --env-file .env -f deploy/compose.yaml
# Console consumer получает собственную случайную группу; offsets приложений не трогаем.
KAFKA_OBSERVER_ARGS ?=
KAFKA_OBSERVER = $(COMPOSE) exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
	--bootstrap-server kafka:9092 --topic task.events \
	--consumer-property enable.auto.commit=false \
	--property print.timestamp=true --property print.partition=true \
	--property print.offset=true --property print.key=true --property print.value=true
TOOLS := $(CURDIR)/.tools
PROTOC ?= protoc
PATH := $(TOOLS):$(PATH)
export PATH
BINS := api task-worker automation-service migrate healthcheck
.PHONY: help env build test test-race vet fmt ci tools proto docker-build up down restart ps logs logs-api logs-worker logs-automation logs-kafka migrate-up migrate-down migrate-status psql redis-cli kafka-topics kafka-consume grpc-list demo test-db test-db-race test-redis test-kafka integration qa-compose clean
help: ## Show developer commands
	@awk 'BEGIN {FS=":.*## "} /^[a-zA-Z_-]+:.*## / {printf "make %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
env: ## Create local .env once; never overwrite it
	@python3 scripts/create-env.py
build: ## Build three processes and tooling (no Docker needed)
	@mkdir -p bin
	@set -e; for name in $(BINS); do $(GO) build -buildvcs=false -o bin/$$name ./cmd/$$name; done
test: ## Fast unit/component tests without infrastructure
	$(GO) test ./...
test-race: ## Race detector, requires CGO/C compiler
	$(GO) test -race ./...
vet: ## Static checks
	$(GO) vet ./...
fmt: ## Format all Go source files
	@gofmt -w $$(find . -name '*.go' -not -path './.tools/*')
ci: fmt-check test vet test-race build ## Fast CI
# protoc compiler is installed separately; versions in docs/setup.md.
tools: ## Install pinned Go generators and grpcurl into .tools
	@mkdir -p $(TOOLS)
	GOBIN=$(TOOLS) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5
	GOBIN=$(TOOLS) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	GOBIN=$(TOOLS) $(GO) install github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3
proto: ## Generate pkg/proto/task/v1 (never edit pb.go)
	@mkdir -p pkg/proto
	cd api/proto && $(PROTOC) -I . --go_out=../../pkg/proto --go_opt=paths=source_relative --go-grpc_out=../../pkg/proto --go-grpc_opt=paths=source_relative task/v1/task.proto
docker-build: env ## Build separate images from the shared multi-stage Dockerfile
	$(COMPOSE) build
up: env ## Start clean/local stack, migrations and topic bootstrap
	$(COMPOSE) up --build -d --wait --wait-timeout 180
down: env ## Stop containers; PostgreSQL/Kafka volumes remain
	$(COMPOSE) down
restart: env ## Restart application processes, keep infrastructure
	$(COMPOSE) restart api-service task-worker automation-service
ps: env ## Show services and health
	$(COMPOSE) ps -a
logs: env ## Follow logs of all processes
	$(COMPOSE) logs -f --tail=100
logs-api: env ## API stdout
	$(COMPOSE) logs -f --tail=100 api-service
logs-worker: env ## Worker stdout
	$(COMPOSE) logs -f --tail=100 task-worker
logs-automation: env ## Automation stdout
	$(COMPOSE) logs -f --tail=100 automation-service
logs-kafka: env ## Broker stdout
	$(COMPOSE) logs -f --tail=100 kafka
migrate-up: env ## Apply pending migrations, journaled and idempotent
	$(COMPOSE) run --rm migrate /app/migrate up
migrate-down: env ## Roll back only newest migration (read cutover notes first)
	$(COMPOSE) run --rm migrate /app/migrate down
migrate-status: env ## Goose migration status
	$(COMPOSE) run --rm migrate /app/migrate status
psql: env ## Open owner service database
	$(COMPOSE) exec postgres psql -U postgres -d task_api
redis-cli: env ## Inspect refresh hashes and TTL, use SCAN
	$(COMPOSE) exec redis redis-cli
kafka-topics: env ## Describe partitions of task.events
	$(COMPOSE) exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --describe --topic task.events
kafka-messages: env ## Show retained Kafka messages, then follow new ones (Ctrl+C to stop)
	$(KAFKA_OBSERVER) --from-beginning $(KAFKA_OBSERVER_ARGS)
kafka-messages-new: env ## Follow only new Kafka messages after observer assignment
	$(KAFKA_OBSERVER) --consumer-property auto.offset.reset=latest $(KAFKA_OBSERVER_ARGS)
kafka-consume: kafka-messages ## Compatibility alias for kafka-messages
# Reflection is for trusted local demo only.
grpc-list: ## List gRPC service with local grpcurl
	$(TOOLS)/grpcurl -plaintext localhost:50051 list
demo: ## HTTP login/refresh, automation gRPC create/get, worker result
	bash scripts/demo.sh
test-db: ## PostgreSQL adapter tests only, TEST_DATABASE_URL required
	$(GO) test -tags=integration -count=1 ./internal/repository/postgres ./internal/migration
test-db-race: ## Same PostgreSQL integration with race
	$(GO) test -race -tags=integration -count=1 ./internal/repository/postgres ./internal/migration
test-redis: ## Real Redis lifecycle/TTL/concurrent rotation (TEST_REDIS_URL)
	$(GO) test -race -tags=integration -count=1 ./internal/repository/redis
test-kafka: ## Real Kafka publish/worker/result (TEST_KAFKA_BROKERS)
	$(GO) test -race -tags=integration -count=1 ./internal/messaging/kafka
integration: test-db test-redis test-kafka ## All real infrastructure integration tests
qa-compose: ## Disposable cold-stack QA (requires free ports, own volumes only)
	bash scripts/compose-qa.sh
clean: ## Remove local build/tools; does not delete volumes or generated contract
	rm -rf bin .tools

fmt-check: ## Refuse unformatted Go code
	@test -z "$$(gofmt -l $$(find cmd internal pkg test -name '*.go'))"
compose-config: env ## Validate Compose without starting containers
	$(COMPOSE) config --quiet
migrate-local-up: ## Native goose up; DATABASE_URL must be exported
	$(GO) run ./cmd/migrate up
migrate-local-down: ## Native goose down by one version
	$(GO) run ./cmd/migrate down
migrate-local-status: ## Native goose status
	$(GO) run ./cmd/migrate status
structure: env ## Schema snapshot from Compose PostgreSQL
	$(COMPOSE) exec -T postgres pg_dump -U postgres -d task_api --schema-only --no-owner --no-privileges > structure.sql
structure-local: ## Schema snapshot using exported DATABASE_URL and pg_dump
	bash scripts/dump-structure.sh
status: migrate-status ## Compatibility alias; stack state uses make ps
curl-demo: ## Previous HTTP CRUD lifecycle
	bash scripts/curl-demo.sh
session-demo: ## Previous access expiration/refresh/logout lifecycle; use 20s Access TTL
	bash scripts/session-demo.sh

.PHONY: help env build test test-race vet fmt ci tools proto docker-build up down restart ps logs logs-api logs-worker logs-automation logs-kafka migrate-up migrate-down migrate-status psql redis-cli kafka-topics kafka-consume kafka-messages kafka-messages-new grpc-list demo test-db test-db-race test-redis test-kafka integration qa-compose clean fmt-check compose-config migrate-local-up migrate-local-down migrate-local-status structure structure-local status curl-demo session-demo
