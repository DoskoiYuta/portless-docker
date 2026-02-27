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

## ─── デフォルト ────────────────────────────────────────

all: build  ## バイナリをビルド（デフォルト）

## ─── ビルド ────────────────────────────────────────────

build:  ## 現在のプラットフォーム向けにビルド
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "ビルド完了: $(DIST_DIR)/$(BINARY_NAME)"

build-all: clean  ## 全プラットフォーム向けにクロスコンパイル
	GOOS=darwin  GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64  $(CMD_DIR)
	GOOS=darwin  GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64  $(CMD_DIR)
	GOOS=linux   GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64   $(CMD_DIR)
	GOOS=linux   GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64   $(CMD_DIR)
	@echo "クロスコンパイル完了。バイナリは $(DIST_DIR)/ にあります"

install: build  ## $GOPATH/bin にインストール
	cp $(DIST_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "インストール完了: $(GOPATH)/bin/$(BINARY_NAME)"

## ─── テスト ────────────────────────────────────────────

test:  ## テストを実行
	$(GO) test ./...

test-verbose:  ## 詳細出力でテストを実行
	$(GO) test -v ./...

test-cover:  ## カバレッジレポート付きでテストを実行
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@echo "HTMLレポート: go tool cover -html=coverage.out"

## ─── 品質 ──────────────────────────────────────────────

lint:  ## golangci-lint を実行（インストール: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest）
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint が見つかりません。インストール: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

fmt:  ## コードをフォーマット
	$(GO) fmt ./...

vet:  ## go vet を実行
	$(GO) vet ./...

tidy:  ## 依存関係を整理・検証
	$(GO) mod tidy
	$(GO) mod verify

## ─── 実行 ──────────────────────────────────────────────

run: build  ## ビルドして引数付きで実行（例: make run ARGS="ls"）
	$(DIST_DIR)/$(BINARY_NAME) $(ARGS)

## ─── クリーン ──────────────────────────────────────────

clean:  ## ビルド成果物を削除
	rm -rf $(DIST_DIR)
	rm -f coverage.out

## ─── ヘルプ ────────────────────────────────────────────

help:  ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
