# portless-docker

Docker Compose 環境向けの `.localhost` サブドメインルーター。

[vercel/portless](https://github.com/vercel-labs/portless) にインスパイアされた Docker Compose 特化版です。

> **Note**: このプロジェクトは 100% vibe coding で作られています。設計からコーディング、テスト、CI 構築まで、すべて AI との対話によって生成されました。

## 解決する課題

| 課題 | 従来 | portless-docker |
|------|------|-----------------|
| ポート番号の記憶 | `localhost:3000` は何？ | `frontend.myapp.localhost` で明確 |
| 複数プロジェクトのポート競合 | `EADDRINUSE` | 自動で空きポートに振り分け |
| Cookie / Storage 汚染 | `localhost` 上で混在 | サブドメインごとに分離 |
| チームへの影響 | 設定ファイル追加が必要 | **追加ファイルなし** |

## 仕組み

```
Browser: http://frontend.myapp.localhost:1355
                    │
         .localhost → 127.0.0.1 (RFC 6761, DNS 設定不要)
                    │
        portless-docker proxy (:1355)
         frontend.myapp.localhost → :41023
         api.myapp.localhost      → :41056
                │                │
          Docker frontend  Docker api
           :41023 (内部3000) :41056 (内部8080)
```

1. `docker-compose.yml` からサービスとポートを自動検出
2. 空きポート（40000-49999）を動的に割り当て
3. 一時オーバーライドファイルを `/tmp` に生成
4. ポート 1355 のリバースプロキシで `<service>.<project>.localhost` をルーティング
5. `docker compose` を元ファイル + オーバーライドで実行
6. 終了時に自動クリーンアップ

### HTTP サービスと TCP サービスの自動判定

コンテナポートが以下の well-known TCP ポートに該当する場合、HTTP プロキシではなく直接ポートマッピングで公開されます:

| ポート | サービス |
|--------|----------|
| 3306 | MySQL |
| 5432 | PostgreSQL |
| 6379 / 6380 | Redis |
| 26379 | Redis Sentinel |
| 27017 | MongoDB |
| 9042 | Cassandra |
| 5672 | RabbitMQ (AMQP) |
| 11211 | Memcached |
| 2181 | ZooKeeper |
| 9092 | Kafka |

それ以外のポート（80, 3000, 8080 など）は HTTP サービスとして `<service>.<project>.localhost:1355` 経由でアクセスできます。`<project>` はディレクトリ名から自動的に決定されます（アンダースコアはハイフンに、大文字は小文字に変換）。

## 前提条件

- **Go 1.22+** — [ダウンロード](https://go.dev/dl/)
- **Docker** + `docker compose` v2 — [インストール](https://docs.docker.com/get-docker/)

## インストール

### バイナリをダウンロード（推奨）

[Releases ページ](https://github.com/DoskoiYuta/portless-docker/releases/latest)からお使いの OS・アーキテクチャに合ったバイナリをダウンロードできます。

```bash
# 例: macOS (Apple Silicon)
curl -L https://github.com/DoskoiYuta/portless-docker/releases/latest/download/portless-docker-darwin-arm64 -o portless-docker
chmod +x portless-docker
sudo mv portless-docker /usr/local/bin/
```

### go install

```bash
go install github.com/DoskoiYuta/portless-docker/cmd/portless-docker@latest
```

### ソースからビルド

```bash
git clone https://github.com/DoskoiYuta/portless-docker.git
cd portless-docker
make build
sudo cp dist/portless-docker /usr/local/bin/
```

## クイックスタート

`docker-compose.yml` があるディレクトリで:

```bash
# サービスを起動（docker compose up の代わり）
portless-docker up

# ブラウザでアクセス（ディレクトリ名が myapp の場合）:
#   http://frontend.myapp.localhost:1355
#   http://api.myapp.localhost:1355
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
| `{{service.host}}` | `service.project.localhost` |
| `{{service.port}}` | プロキシのリッスンポート（HTTP）/ 割り当てポート（TCP） |
| `{{service.url}}` | `http://service.project.localhost:1355`（HTTP）/ `localhost:4xxxx`（TCP） |
| `{{proxy.port}}` | portless-docker プロキシのリッスンポート（デフォルト `1355`） |

`proxy` は予約名です。`-p` フラグでプロキシポートを変更した場合も `{{proxy.port}}` に反映されます。

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
127.0.0.1  frontend.myapp.localhost
127.0.0.1  api.myapp.localhost
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
