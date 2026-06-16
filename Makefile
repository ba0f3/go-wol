.PHONY: all build test vet fmt lint install-lint help

BINARY := go-wol
GOBIN := $(shell go env GOPATH)/bin
GOLANGCI_LINT := $(firstword $(shell command -v golangci-lint 2>/dev/null) $(GOBIN)/golangci-lint)

all: fmt vet lint test build

build:
	mkdir -p bin
	go build -o bin/$(BINARY) .

test:
	go test -count=1 ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

lint:
	@test -x "$(GOLANGCI_LINT)" || { \
		echo "golangci-lint not found; run: make install-lint"; \
		exit 1; \
	}
	$(GOLANGCI_LINT) run ./...

install-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

help:
	@echo "Targets:"
	@echo "  all          fmt, vet, lint, test, build"
	@echo "  build        compile $(BINARY) binary"
	@echo "  test         run unit tests"
	@echo "  vet          go vet"
	@echo "  fmt          go fmt ./..."
	@echo "  lint         golangci-lint run"
	@echo "  install-lint install golangci-lint to GOPATH/bin"
