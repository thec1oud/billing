.DEFAULT_GOAL := help
.PHONY: help setup up down logs fmt vet lint pre-commit-install sqlc-diff sqlc-generate test test-race test-integration ci web-dev web-build bff-dev bff-test

help:
	@echo "Available commands:"
	@echo "  make help                 Show this command list (default)"
	@echo "  make setup                Install dependencies and start local Docker services"
	@echo "  make up                   Build and start the local development stack"
	@echo "  make down                 Stop the local development stack"
	@echo "  make logs                 Follow local Docker service logs"
	@echo "  make fmt                  Format Go source files"
	@echo "  make vet                  Run go vet"
	@echo "  make lint                 Run golangci-lint"
	@echo "  make test                 Run Go unit tests"
	@echo "  make test-race            Run Go tests with the race detector"
	@echo "  make test-integration     Run Go integration tests"
	@echo "  make sqlc-diff            Check generated SQL code for drift"
	@echo "  make sqlc-generate        Regenerate SQL code"
	@echo "  make ci                   Run SQL, vet, lint, Go race, and BFF checks"
	@echo "  make web-dev              Start the Vite web development server"
	@echo "  make web-build            Type-check and build the web application"
	@echo "  make bff-dev              Start the BFF development server"
	@echo "  make bff-test             Run BFF tests"
	@echo "  make pre-commit-install   Install the repository's pre-commit hooks"

setup:
	node scripts/setup.mjs

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

fmt:
	gofmt -s -w .
	goimports -w . || true

vet:
	go vet ./...

lint:
	golangci-lint run ./...

pre-commit-install:
	pre-commit install

sqlc-diff:
	sqlc diff

sqlc-generate:
	sqlc generate

test:
	go test ./...

test-race:
	go test -v -race ./...

test-integration:
	go test -tags=integration -v ./internal/integration/...

ci:
	$(MAKE) sqlc-diff
	$(MAKE) vet
	$(MAKE) lint
	$(MAKE) test-race
	$(MAKE) bff-test

web-dev:
	cd platform/web && npm run dev

web-build:
	cd platform/web && npm run build

bff-dev:
	cd platform/bff && npm run dev

bff-test:
	cd platform/bff && npm test
