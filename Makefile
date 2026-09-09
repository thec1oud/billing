.PHONY: fmt lint pre-commit-install ci web-dev web-build

fmt:
	gofmt -s -w .
	goimports -w . || true

lint:
	golangci-lint run ./...

pre-commit-install:
	pre-commit install

ci:
	go vet ./...
	golangci-lint run ./...
	go test ./...

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build
