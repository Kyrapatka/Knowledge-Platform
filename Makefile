.DEFAULT_GOAL := check

GO ?= go
ifeq ($(OS),Windows_NT)
NPM ?= npm.cmd
else
NPM ?= npm
endif
COMPOSE ?= docker compose

.PHONY: run fmt fmt-check vet test test-race test-cover check docker-up docker-down docker-ps docker-logs migrate-up migrate-down migrate-status migrate-version migrate-test dev test-integration frontend-check test-e2e frontend-e2e test-all

run:
	$(GO) run ./cmd/api

fmt:
	$(GO) fmt ./...

fmt-check:
	$(GO) run ./cmd/fmt-check

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-cover:
	$(GO) test -coverprofile=coverage.out ./...

check:
	$(MAKE) fmt
	$(MAKE) vet
	$(MAKE) test
	$(MAKE) test-race

docker-up:
	$(COMPOSE) up -d --build

docker-down:
	$(COMPOSE) down

docker-ps:
	$(COMPOSE) ps

docker-logs:
	$(COMPOSE) logs --follow --tail=100

# The runner loads DATABASE_URL from the environment or .env without shell DSN expansion.
migrate-up:
	$(GO) run ./cmd/migrate up

migrate-down:
	$(GO) run ./cmd/migrate down

migrate-status:
	$(GO) run ./cmd/migrate status

migrate-version: migrate-status

migrate-test:
	$(GO) run ./cmd/migrate smoke

# Compose includes the backend and runs migrations before starting it.
dev: docker-up

test-integration:
	$(GO) run ./cmd/test-integration

frontend-check:
	$(NPM) --prefix web run check

.PHONY: openapi-check
openapi-check:
	$(NPM) --prefix web run openapi:check

test-e2e:
	$(NPM) --prefix web run test:e2e

frontend-e2e: test-e2e

test-all:
	$(MAKE) check
	$(MAKE) test-integration
	$(MAKE) migrate-test
	$(MAKE) frontend-check
	$(MAKE) test-e2e


.PHONY: deploy-local rollback-local deployment-status observability grafana observability-down observability-check observability-full-check

deploy-local:
	$(GO) run ./cmd/local deploy

rollback-local:
	$(GO) run ./cmd/local rollback

deployment-status:
	$(GO) run ./cmd/local status

observability:
	$(GO) run ./cmd/local observability

grafana: observability

observability-down:
	$(GO) run ./cmd/local down

observability-check:
	$(GO) run ./cmd/local check

observability-full-check:
	$(GO) run ./cmd/local full-check
