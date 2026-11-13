.PHONY: fmt vet lint pre-commit-install sqlc-diff sqlc-generate test test-race test-integration ci web-dev web-build bff-dev bff-test

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
