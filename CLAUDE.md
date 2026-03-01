# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

portless-docker は Go 製の CLI ツールで、Docker Compose 環境向けの `.localhost` サブドメインルーターです。`docker-compose.yml` からポートマッピングを自動検出し、動的ホストポート（40000-49999）を割り当て、一時的なオーバーライドファイルを生成し、ポート 1355 でリバースプロキシ経由のルーティングを行います。

## Build & Development Commands

```bash
make build          # ビルド → dist/portless-docker
make install        # ビルドして $GOPATH/bin にインストール
make test           # 全テスト実行 (go test ./...)
make test-verbose   # 詳細出力付きテスト
make test-cover     # カバレッジレポート
make lint           # golangci-lint 実行
make fmt            # go fmt でフォーマット
make vet            # go vet 実行
make tidy           # go mod tidy & verify
make build-all      # クロスコンパイル (Darwin/Linux × amd64/arm64)
```

単一テスト実行: `go test ./internal/compose/ -run TestParsePorts -v`

## Architecture

```
CLI (Cobra) → Compose Parser → Port Allocator → Override Generator → Proxy Daemon → State Manager
```

### パッケージ構成

- **`internal/cli/`** — Cobra ベースの CLI コマンド。`passthrough.go` が中核で、未知のコマンドを `docker compose` にパススルーしつつ、ポート割り当て・オーバーライド生成・プロキシ起動を行う
- **`internal/compose/`** — `docker-compose.yml` のパース（`parse.go`）、`portless-docker.env.*` ラベルからの環境変数テンプレート解決（`envlabel.go`）、一時オーバーライドファイル生成（`override.go`）
- **`internal/ports/`** — ポート割り当て。ランダム割り当てと FNV-1a ハッシュによる決定的割り当ての 2 戦略
- **`internal/proxy/`** — `<service>.localhost:1355` へのリクエストを割り当てポートにルーティングするリバースプロキシ（デーモン管理含む）
- **`internal/state/`** — `~/.portless-docker/state.json` でルートとプロキシ PID を管理。ファイルベースロックで排他制御
- **`internal/ui/`** — lipgloss を使ったリッチなターミナル出力

### データフロー

`docker-compose.yml` → `ParseComposeFile()` → `AllocateDeterministic()` → `ParseEnvLabels()` + `ResolveEnv()` → `GenerateOverride()` → 一時ファイル `/tmp/portless-docker-XXXX/override.yml` → ルート登録 → プロキシ起動 → `docker compose -f original -f override up`

## Code Style & Conventions

- Go 1.24.7、コメント・ドキュメントは日本語
- golangci-lint 設定: errcheck, govet, staticcheck, unused, misspell, unconvert, unparam
- テストはテーブル駆動スタイル
- CI（GitHub Actions）で gofmt チェック、go vet、lint、race 検出付きテスト、クロスコンパイルを実施
