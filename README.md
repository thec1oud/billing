# Gebeta Billing

## Development setup

### Install dependencies

```powershell
go env -w GOPROXY=https://proxy.golang.org,direct
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/tools/cmd/goimports@latest
pre-commit install
```

### Common commands

```powershell
go test ./...
gofmt -s -w .
goimports -w .
golangci-lint run ./...
make fmt
make lint
make ci
```

### Recommended workflow

1. Run `make fmt`
2. Run `make lint`
3. Run `go test ./...`
4. Commit and push
