BINARY_NAME := portless-docker
MODULE      := github.com/DoskoiYuta/portless-docker
CMD_DIR     := ./cmd/portless-docker
DIST_DIR    := dist

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS     := -s -w \
	-X '$(MODULE)/internal/cli.Version=$(VERSION)' \
	-X '$(MODULE)/internal/cli.Commit=$(COMMIT)'

GO          := go
GOFLAGS     := -trimpath

.PHONY: all build build-all install clean test test-verbose test-cover lint fmt vet tidy run help

## ─── Default ───────────────────────────────────────────

all: build  ## Build the binary (default)

## ─── Build ─────────────────────────────────────────────

build:  ## Build for current platform
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)"

build-all: clean  ## Cross-compile for all platforms
	GOOS=darwin  GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64  $(CMD_DIR)
	GOOS=darwin  GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64  $(CMD_DIR)
	GOOS=linux   GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64   $(CMD_DIR)
	GOOS=linux   GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64   $(CMD_DIR)
	@echo "Cross-compilation complete. Binaries in $(DIST_DIR)/"

install: build  ## Install to $GOPATH/bin
	cp $(DIST_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

## ─── Test ──────────────────────────────────────────────

test:  ## Run tests
	$(GO) test ./...

test-verbose:  ## Run tests with verbose output
	$(GO) test -v ./...

test-cover:  ## Run tests with coverage report
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@echo "HTML report: go tool cover -html=coverage.out"

## ─── Quality ───────────────────────────────────────────

lint:  ## Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not found. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

fmt:  ## Format code
	$(GO) fmt ./...

vet:  ## Run go vet
	$(GO) vet ./...

tidy:  ## Tidy and verify dependencies
	$(GO) mod tidy
	$(GO) mod verify

## ─── Run ───────────────────────────────────────────────

run: build  ## Build and run with arguments (e.g., make run ARGS="ls")
	$(DIST_DIR)/$(BINARY_NAME) $(ARGS)

## ─── Clean ─────────────────────────────────────────────

clean:  ## Remove build artifacts
	rm -rf $(DIST_DIR)
	rm -f coverage.out

## ─── Help ──────────────────────────────────────────────

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
