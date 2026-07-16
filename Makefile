.PHONY: fmt lint pre-commit-install ci

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
