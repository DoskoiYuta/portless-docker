# portless-docker

Docker Compose 環境向けの `.localhost` サブドメインルーター。

[vercel/portless](https://github.com/vercel-labs/portless) にインスパイアされた Docker Compose 特化版です。

> **Note**: このプロジェクトは 100% vibe coding で作られています。設計からコーディング、テスト、CI 構築まで、すべて AI との対話によって生成されました。

## 解決する課題

| 課題 | 従来 | portless-docker |
|------|------|-----------------|
| ポート番号の記憶 | `localhost:3000` は何？ | `frontend.localhost` で明確 |
| 複数プロジェクトのポート競合 | `EADDRINUSE` | 自動で空きポートに振り分け |
| Cookie / Storage 汚染 | `localhost` 上で混在 | サブドメインごとに分離 |
| チームへの影響 | 設定ファイル追加が必要 | **追加ファイルなし** |

## 仕組み

```
Browser: http://frontend.localhost:1355
                    │
         .localhost → 127.0.0.1 (RFC 6761, DNS 設定不要)
                    │
        portless-docker proxy (:1355)
         frontend.localhost → :41023
         api.localhost      → :41056
                │                │
          Docker frontend  Docker api
           :41023 (内部3000) :41056 (内部8080)
```

1. `docker-compose.yml` からサービスとポートを自動検出
2. 空きポート（40000-49999）を動的に割り当て
3. 一時オーバーライドファイルを `/tmp` に生成
4. ポート 1355 のリバースプロキシで `<service>.localhost` をルーティング
5. `docker compose` を元ファイル + オーバーライドで実行
6. 終了時に自動クリーンアップ

## 前提条件

- **Go 1.22+** — [ダウンロード](https://go.dev/dl/)
- **Docker** + `docker compose` v2 — [インストール](https://docs.docker.com/get-docker/)

## インストール

```bash
# ソースからビルド
git clone https://github.com/DoskoiYuta/portless-docker.git
cd portless-docker
make build
# → dist/portless-docker

# PATH に配置
sudo cp dist/portless-docker /usr/local/bin/

# または go install
go install github.com/DoskoiYuta/portless-docker/cmd/portless-docker@latest
```

## クイックスタート

`docker-compose.yml` があるディレクトリで:

```bash
# サービスを起動（docker compose up の代わり）
portless-docker up

# ブラウザでアクセス:
#   http://frontend.localhost:1355
#   http://api.localhost:1355
```

## 使い方

```bash
# フォアグラウンド起動（Ctrl+C で停止・自動クリーンアップ）
portless-docker up

# バックグラウンド起動
portless-docker up -d

# アクティブルート一覧
portless-docker ls

# サービスへのコマンド実行
portless-docker exec api bash
portless-docker run api rails db:migrate
portless-docker logs -f frontend

# サービス停止
portless-docker down

# 全プロジェクト停止
portless-docker stop --all
```

### グローバルフラグ

```bash
# プロキシポートを変更
portless-docker -p 8888 up

# Compose ファイルを指定
portless-docker -f compose.prod.yml up

# 特定のサービスを除外
portless-docker --ignore redis,postgres up -d
```

### パススルー

`ls`、`stop`、`--help`、`--version` 以外のサブコマンドはすべて `docker compose` にそのまま透過されます。

```bash
portless-docker build --no-cache
portless-docker restart api
portless-docker ps
```

## 環境変数テンプレート

Docker Compose ラベルで、サービス間の動的な環境変数を設定できます:

```yaml
services:
  frontend:
    ports:
      - "3000:3000"
    labels:
      portless-docker.env.API_URL: "http://{{api.host}}:{{api.port}}"
  api:
    ports:
      - "8080:8080"
```

テンプレートプレースホルダー:

| プレースホルダー | 値 |
|---------------|---|
| `{{service.host}}` | `service.localhost` |
| `{{service.port}}` | プロキシのリッスンポート |
| `{{service.url}}` | `http://service.localhost:1355` |

## 状態ファイル

`~/.portless-docker/` に保存:

| ファイル | 用途 |
|---------|------|
| `state.json` | アクティブルート |
| `proxy.pid` | プロキシの PID |
| `proxy.port` | プロキシのポート |
| `proxy.log` | プロキシのログ |

## トラブルシューティング

```bash
# プロキシが起動しない場合
cat ~/.portless-docker/proxy.log

# ポート 1355 が使用中の場合
portless-docker -p 9999 up

# クラッシュ後の状態リセット
portless-docker stop --all
```

`.localhost` がブラウザで解決しない場合は `/etc/hosts` に追加:

```
127.0.0.1  frontend.localhost
127.0.0.1  api.localhost
```

## 開発

```bash
make fmt            # コードフォーマット
make vet            # 静的解析
make test           # テスト実行
make lint           # golangci-lint
make build          # ビルド
make build-all      # クロスコンパイル (Darwin/Linux × amd64/arm64)
```

## ライセンス

MIT
