SHELL := /bin/bash
GOCMD := go
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOBUILD := $(GOCMD) build

.PHONY: help run build test test-unit test-integration test-e2e lint fmt vet vuln openapi tidy tools migrate-up migrate-down migrate-status migrate-create docker-build docker-up docker-down docker-clean ci cover clean

help:
	@grep -E '^[a-zA-Z0-9_/-]+:.*##' Makefile | sort | sed 's/:.*##/\t/'

run: ## run the API locally
	$(GOCMD) run ./cmd/api

build: ## build the binary
	$(GOBUILD) ./...

test: ## run unit tests
	$(MAKE) test-unit

test-unit: ## run unit and handler tests with race detection
	$(GOTEST) -race -shuffle=on ./...

test-integration: ## run Postgres repository integration tests
	docker compose -f deploy/docker-compose.yml up -d db
	DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/appdb?sslmode=disable}" $(GOCMD) run ./cmd/migrate -- up
	DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/appdb?sslmode=disable}" $(GOTEST) -race -shuffle=on -tags=integration ./internal/integration

test-e2e: ## run the curl end-to-end flow against Compose
	docker compose -f deploy/docker-compose.yml up --build -d
	bash scripts/e2e.sh

lint: ## run golangci-lint
	golangci-lint run

vuln: ## scan dependencies with govulncheck
	govulncheck ./...

openapi: ## validate the OpenAPI contract
	npx --yes @redocly/cli lint api/openapi.yaml

vet: ## run go vet
	$(GOVET) ./...

fmt: ## format Go code
	$(GOCMD) fmt ./...

cover: ## print coverage summary
	$(GOCMD) test -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -func=coverage.out

migrate-up: ## run migrations up
	$(GOCMD) run ./cmd/migrate -- up

migrate-down: ## run migrations down
	$(GOCMD) run ./cmd/migrate -- down

migrate-status: ## show migration status
	$(GOCMD) run ./cmd/migrate -- status

migrate-create: ## create a migration file
	$(GOCMD) run ./cmd/migrate -- create $(name)

docker-build: ## build docker image
	docker build -f deploy/Dockerfile -t prod-go:latest .

docker-up: ## start docker compose stack
	docker compose -f deploy/docker-compose.yml up --build -d

docker-down: ## stop docker compose stack
	docker compose -f deploy/docker-compose.yml down

docker-clean: ## remove Compose stack and its persistent database volume
	docker compose -f deploy/docker-compose.yml down -v

ci: ## run full CI checks locally
	$(GOCMD) fmt ./...
	$(GOBUILD) ./...
	$(GOCMD) vet ./...
	golangci-lint run
	govulncheck ./...
	$(GOTEST) -race -shuffle=on ./...
	$(MAKE) openapi
	$(MAKE) cover

clean: ## remove build artifacts
	rm -rf coverage.out dist/

tidy: ## tidy modules
	$(GOCMD) mod tidy

tools: ## install local dev tools
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
