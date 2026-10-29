SHELL := /bin/bash
GOCMD := go
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOBUILD := $(GOCMD) build

.PHONY: help build test lint vet fmt tidy tools migrate-up migrate-down migrate-status migrate-create docker-build docker-up docker-down ci cover run

help:
	@grep -E '^[a-zA-Z0-9_/-]+:.*##' Makefile | sort | sed 's/:.*##/\t/'

run: ## run the API locally
	$(GOCMD) run ./cmd/api

build: ## build the binary
	$(GOBUILD) ./...

test: ## run unit tests
	$(GOTEST) ./...

lint: ## run golangci-lint
	golangci-lint run

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
	docker compose -f deploy/docker-compose.yml up -d

docker-down: ## stop docker compose stack
	docker compose -f deploy/docker-compose.yml down -v

ci: ## run full CI checks locally
	$(GOCMD) fmt ./...
	$(GOCMD) vet ./...
	golangci-lint run
	$(GOCMD) test -race -shuffle=on ./...

clean: ## remove build artifacts
	rm -rf coverage.out dist/

tidy: ## tidy modules
	$(GOCMD) mod tidy

tools: ## install local dev tools
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
