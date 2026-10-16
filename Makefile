.PHONY: fmt lint pre-commit-install sqlc-diff sqlc-generate test test-race ci web-dev web-build bff-dev bff-test

fmt:
	gofmt -s -w .
	goimports -w . || true

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
	go test -race ./...

ci:
	sqlc diff
	go vet ./...
	golangci-lint run ./...
	go test -race $$(go list ./... | grep -v 'github.com/thec1oud/billing/internal/integration')
	cd platform/bff && npm test

web-dev:
	cd platform/web && npm run dev

web-build:
	cd platform/web && npm run build

bff-dev:
	cd platform/bff && npm run dev

bff-test:
	cd platform/bff && npm test
