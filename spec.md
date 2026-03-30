# portless-docker 設計書

## 1. 概要

portless-docker は、Docker Compose 環境に特化した `.localtest.me` サブドメインルーターである。

`docker-compose.yml` の `ports` 定義を自動検出し、ホスト側ポートを空きポートに動的に差し替えることで、複数プロジェクトのポート衝突を解消する。設定ファイルは不要。`docker compose` の代わりに `portless-docker` と打つだけで動作する。

Go のシングルバイナリとして配布し、Docker / Docker Compose 以外のランタイム依存なしで動作する。

---

## 2. 解決する課題

| 課題 | 現状 | portless-docker 導入後 |
|------|------|----------------------|
| ポート番号の記憶 | `localhost:3000` は何のサービス？ | `frontend.localtest.me` で明確 |
| 複数プロジェクトのポート競合 | Project A も B も 3000 番で `EADDRINUSE` | 自動で空きポートに振り分け |
| Cookie / Storage 汚染 | `localhost` 上でポート違いの Cookie が混在 | サブドメインごとに分離 |
| チームへの影響 | ツール導入で設定ファイル追加が必要 | **追加ファイルなし** |
| AI エージェント連携 | ポートが不安定 | 名前で安定アクセス |

---

## 3. 設計原則

1. **設定ファイル不要** — `docker-compose.yml` から全自動検出
2. **プロジェクトにファイルを追加しない** — override は tmpdir に生成し、終了時に自動削除
3. **docker-compose.yml を変更しない**
4. **ゼロ依存** — Go シングルバイナリ
5. **グローバルデーモン** — 1 つのプロキシプロセスが全プロジェクトのルートを管理
6. **自動ライフサイクル** — プロキシは初回利用時に自動起動し、不要になれば自動終了

---

## 4. 動作の流れ

### 4.1 シーケンス

```
$ portless-docker up

  ① docker-compose.yml を自動検出・パース
     → services.frontend.ports: ["3000:3000"]
     → services.api.ports: ["8080:8080"]
     → services.redis.ports: なし → 対象外

  ② 各サービスに空きポートを割り当て
     → frontend: 41023 (ホスト側) : 3000 (コンテナ内)
     → api:      41056 (ホスト側) : 8080 (コンテナ内)

  ③ 一時 override ファイルを /tmp に生成
     /tmp/portless-docker-XXXXXX/override.yml

  ④ グローバルプロキシにルート登録
     → frontend.localtest.me:1355 → 127.0.0.1:41023
     → api.localtest.me:1355      → 127.0.0.1:41056

  ⑤ プロキシが未起動なら自動起動

  ⑥ docker compose を実行
     docker compose \
       -f docker-compose.yml \
       -f /tmp/portless-docker-XXXXXX/override.yml \
       up

  ⑦ 終了時（Ctrl+C / SIGTERM）
     → 一時 override ファイルを削除
     → ルートを解除
     → 他プロジェクトのルートがなければプロキシも自動停止
```

### 4.2 全体構成図

```
┌─────────────────────────────────────────────────┐
│                    Browser                       │
│  http://frontend.localtest.me:1355                  │
│  http://api.localtest.me:1355                       │
└──────────────────┬──────────────────────────────┘
                   │
        .localtest.me → 127.0.0.1 (パブリック DNS サービス)
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│       portless-docker proxy (port 1355)          │
│                                                  │
│  frontend.localtest.me → 127.0.0.1:41023            │
│  api.localtest.me      → 127.0.0.1:41056            │
└───────┬──────────────────┬──────────────────────┘
        │                  │
        ▼                  ▼
  ┌──────────┐       ┌──────────┐
  │ Docker   │       │ Docker   │
  │ frontend │       │ api      │
  │ :41023   │       │ :41056   │
  │(内部3000) │       │(内部8080) │
  └──────────┘       └──────────┘

※ コンテナ内部のポートは変わらない
※ ホスト側の公開ポートだけを差し替え
```

---

## 5. CLI

### 5.1 サブコマンドの分類

portless-docker のサブコマンドは **独自コマンド** と **パススルーコマンド** の 2 種類に分かれる。

#### 独自コマンド

portless-docker 自身が処理するコマンド。

| コマンド | 説明 |
|----------|------|
| `portless-docker ls` | 全プロジェクトのアクティブルートを一覧表示 |
| `portless-docker stop --all` | 全ルート解除 + プロキシ停止 |
| `portless-docker --help` | ヘルプ表示 |
| `portless-docker --version` | バージョン表示 |

#### パススルーコマンド

上記以外のサブコマンドはすべて `docker compose -f <compose> -f <override>` に透過する。

```
portless-docker <subcmd> [args...]
  ↓
docker compose -f docker-compose.yml -f /tmp/.../override.yml <subcmd> [args...]
```

これにより、`docker compose` のあらゆるサブコマンドが自動的にサポートされる。

### 5.2 パススルーの例

```bash
# up
portless-docker up -d --build
# → docker compose -f ... -f ... up -d --build

# down (+ ルート解除 + override 削除)
portless-docker down --volumes
# → docker compose -f ... -f ... down --volumes

# run
portless-docker run api rails db:migrate
# → docker compose -f ... -f ... run api rails db:migrate

# exec
portless-docker exec api bash
# → docker compose -f ... -f ... exec api bash

# logs
portless-docker logs -f frontend
# → docker compose -f ... -f ... logs -f frontend

# ps
portless-docker ps
# → docker compose -f ... -f ... ps

# restart
portless-docker restart api
# → docker compose -f ... -f ... restart api

# pull, build, config, top, etc.
portless-docker build --no-cache
# → docker compose -f ... -f ... build --no-cache
```

### 5.3 サブコマンドごとの追加動作

ほとんどのサブコマンドは単純にパススルーするが、一部は前後に追加の処理を行う。

| サブコマンド | 前処理 | パススルー | 後処理 |
|------------|--------|-----------|--------|
| `up` | ポート割り当て、override 生成、ルート登録、プロキシ起動 | `docker compose ... up` | (フォアグラウンド終了時) override 削除、ルート解除 |
| `down` | — | `docker compose ... down` | override 削除、ルート解除 |
| その他全て | — | `docker compose ... <subcmd>` | — |

**判定ロジック:**

```
サブコマンドが "ls", "stop", "--help", "--version" ?
  → yes: 独自コマンドとして処理

ルートが未登録（state.json にこのディレクトリのエントリがない）?
  → yes: ポート割り当て + override 生成 + ルート登録 + プロキシ起動
  → no:  state.json から既存の override パスを使用

サブコマンドを docker compose にパススルー

サブコマンドが "down" ?
  → yes: override 削除 + ルート解除 + プロキシ自動停止チェック

サブコマンドが "up" かつフォアグラウンド（-d なし）?
  → yes: 終了時に override 削除 + ルート解除 + プロキシ自動停止チェック
```

### 5.4 グローバルフラグ

| フラグ | デフォルト | 説明 |
|--------|----------|------|
| `-p, --port <number>` | `1355` | プロキシのリッスンポート |
| `-f, --file <path>` | 自動検出 | Compose ファイルのパス |
| `--ignore <services>` | なし | プロキシ対象外のサービス（カンマ区切り） |

グローバルフラグはサブコマンドの前に置く:

```bash
portless-docker --ignore redis,postgres up -d --build
portless-docker -f compose.prod.yml exec api bash
```

---

## 6. コマンド出力例

### `portless-docker up`

```
$ portless-docker up

portless-docker
Compose: /home/user/myproject/docker-compose.yml

  http://frontend.localtest.me:1355  →  :41023 (container :3000)
  http://api.localtest.me:1355       →  :41056 (container :8080)

Running: docker compose -f docker-compose.yml \
  -f /tmp/portless-docker-xxxx/override.yml up

frontend-1  | ready on port 3000
api-1       | listening on port 8080
```

Ctrl+C 時:
```
^C
Stopping containers...
Cleaned up: override removed, 2 route(s) unregistered.
No routes remaining. Proxy stopped.
```

### `portless-docker up -d`

```
$ portless-docker up -d

portless-docker
Compose: /home/user/myproject/docker-compose.yml

  http://frontend.localtest.me:1355  →  :41023 (container :3000)
  http://api.localtest.me:1355       →  :41056 (container :8080)

Containers started in background.
Run "portless-docker down" to stop.
```

### `portless-docker exec api bash`

```
$ portless-docker exec api bash
root@abc123:/app#
```

override が適用された状態でそのまま exec が実行される。既にルート登録済みなら追加の出力なし。

### `portless-docker down`

```
$ portless-docker down

Stopping containers...
Cleaned up: override removed, 2 route(s) unregistered.
No routes remaining. Proxy stopped.
```

### `portless-docker ls`

```
$ portless-docker ls

Active routes:

 /home/user/project-a
   http://frontend.localtest.me:1355  →  :41023 (container :3000)
   http://api.localtest.me:1355       →  :41056 (container :8080)

 /home/user/project-b
   http://web.localtest.me:1355       →  :41102 (container :3000)
   http://worker.localtest.me:1355    →  :41103 (container :9000)
```

---

## 7. docker-compose.yml の自動検出

### 7.1 Compose ファイルの探索

カレントディレクトリから以下の順で検索:

1. `docker-compose.yml`
2. `docker-compose.yaml`
3. `compose.yml`
4. `compose.yaml`

`-f` フラグで明示指定も可能。

### 7.2 ports パース

| 記法 | ホストポート | コンテナポート |
|------|------------|-------------|
| `"3000:3000"` | 3000 | 3000 |
| `"8080:80"` | 8080 | 80 |
| `"3000"` | 3000 | 3000 |
| `"127.0.0.1:3000:3000"` | 3000 | 3000 |
| `"0.0.0.0:3000:3000/tcp"` | 3000 | 3000 |

- `ports` 定義がないサービス → 対象外
- `--ignore` 指定サービス → 対象外
- 1 サービスに複数 ports → 最初の TCP ポートを使用

### 7.3 サブドメイン決定

サービス名をそのままサブドメインに使用。

| サービス名 | URL |
|-----------|-----|
| `frontend` | `http://frontend.localtest.me:1355` |
| `api` | `http://api.localtest.me:1355` |
| `my-service` | `http://my-service.localtest.me:1355` |
| `web_app` | `http://web-app.localtest.me:1355` |

- `_` → `-` に変換
- 大文字 → 小文字に変換

---

## 8. ポート割り当て

### 8.1 仕様

| 項目 | 値 |
|------|-----|
| ホスト側ポート範囲 | 40000 - 49999 |
| 割り当て方法 | OS にバインドを試行して空きを確認 |
| コンテナ内ポート | **変更しない** |

### 8.2 override 生成例

元の `docker-compose.yml`:
```yaml
services:
  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
  api:
    build: ./api
    ports:
      - "8080:8080"
  redis:
    image: redis
```

生成される一時 override:
```yaml
# Auto-generated by portless-docker. DO NOT EDIT.
services:
  frontend:
    ports: !override
      - "41023:3000"
  api:
    ports: !override
      - "41056:8080"
```

---

## 9. プロキシデーモン

### 9.1 ライフサイクル

```
portless-docker <subcmd> (初回)
    │
    ▼
ルート未登録 → ポート割り当て + override 生成 + ルート登録
    │
    ▼
プロキシ起動済み？ ─── yes ──→ パススルー実行
    │ no
    ▼
デーモンをバックグラウンドで起動
    │
    ▼
ヘルスチェック後、パススルー実行

    ... 利用中 ...

portless-docker down
    │
    ▼
パススルー実行 → ルート解除 → ルート数 0 → プロキシ自動停止
```

### 9.2 プロキシの動作

1. `Host` ヘッダーからホスト名を取得
2. `state.json` から一致するルートを検索
3. 一致 → `127.0.0.1:{割り当てポート}` にリバースプロキシ
4. 不一致 → 404（アクティブルート一覧表示）
5. ターゲット無応答 → 502

### 9.3 対応プロトコル

| プロトコル | 対応 |
|-----------|------|
| HTTP/1.1 | ✅ |
| WebSocket | ✅ |
| HTTP/2 | 将来 |
| HTTPS | 将来 |

### 9.4 付与ヘッダー

| ヘッダー | 値 |
|---------|-----|
| `X-Forwarded-For` | クライアント IP |
| `X-Forwarded-Proto` | `http` |
| `X-Forwarded-Host` | 元の Host |
| `X-Portless-Docker` | `1` |

### 9.5 セルフチェック

30 秒間隔で `state.json` のルート数を監視。0 件が継続したら自動 shutdown。

---

## 10. 状態管理

### 10.1 ディレクトリ

```
~/.portless-docker/
├── state.json
├── proxy.pid
├── proxy.port
├── proxy.log
└── state.lock/
```

### 10.2 state.json

```json
{
  "proxyPort": 1355,
  "routes": [
    {
      "hostname": "frontend.localtest.me",
      "hostPort": 41023,
      "containerPort": 3000,
      "service": "frontend",
      "directory": "/home/user/project-a",
      "composeFile": "/home/user/project-a/docker-compose.yml",
      "overridePath": "/tmp/portless-docker-abc123/override.yml",
      "detached": false,
      "registeredAt": "2026-02-27T10:00:00Z"
    }
  ]
}
```

### 10.3 排他制御

- `mkdir` ベースのファイルロック
- タイムアウト 5 秒、リトライ 50 ms
- 10 秒超のロック → stale として強制削除

### 10.4 競合検出

- 同一ホスト名 + 異なるディレクトリ → エラー
- 同一ディレクトリの再登録 → 上書き

---

## 11. 一時ファイル管理

| 項目 | 仕様 |
|------|------|
| 生成場所 | `/tmp/portless-docker-XXXXXX/override.yml` |
| フォアグラウンド `up` 終了時 | 即削除 |
| デタッチモード | `down` 時に削除 |
| 異常終了時 | 次回実行時にコンテナ状態を確認し、停止していれば自動削除 |

---

## 12. エラーハンドリング

### 12.1 HTTP レスポンス

| 状況 | ステータス | 内容 |
|------|----------|------|
| ルート不一致 | 404 | アクティブルート一覧 |
| コンテナ未起動 | 502 | サービス名、ポート、起動中メッセージ |
| Host ヘッダーなし | 400 | "Missing Host header" |

### 12.2 CLI エラー

| 状況 | メッセージ |
|------|----------|
| Compose ファイルなし | `No docker-compose.yml found.` |
| docker compose 未インストール | `docker compose is not installed or not in PATH.` |
| ports 定義なし | `No services with port mappings found.` |
| ホスト名競合 | `"frontend.localtest.me" is already registered by /other/project` |
| プロキシ起動失敗 | `Proxy failed to start. Check ~/.portless-docker/proxy.log` |

---

## 13. 利用パターン

### 13.1 基本

```bash
portless-docker up
# Ctrl+C → 全自動クリーンアップ
```

### 13.2 デタッチ + 作業 + 停止

```bash
portless-docker up -d
portless-docker exec api bash
portless-docker logs -f frontend
portless-docker down
```

### 13.3 複数プロジェクト

```bash
cd ~/project-a && portless-docker up -d
cd ~/project-b && portless-docker up -d
portless-docker ls
cd ~/project-a && portless-docker down
cd ~/project-b && portless-docker down
```

### 13.4 マイグレーション実行

```bash
portless-docker run api rails db:migrate
```

### 13.5 ビルド

```bash
portless-docker build --no-cache
```

---

## 14. チームへの影響

| 対象 | 影響 |
|------|------|
| `docker-compose.yml` | **変更なし** |
| プロジェクトディレクトリ | **ファイル追加なし** |
| チームメンバー（ツール未導入） | `docker compose up` がそのまま動く |
| CI/CD | 影響なし |
| `.gitignore` | 変更不要 |

---

## 15. 実装構成（Go）

```
portless-docker/
├── cmd/
│   └── portless-docker/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── root.go             # サブコマンド振り分け
│   │   ├── ls.go               # portless-docker ls
│   │   ├── stop.go             # portless-docker stop --all
│   │   └── passthrough.go      # パススルー処理（up/down/その他共通）
│   ├── compose/
│   │   ├── parse.go            # docker-compose.yml パース
│   │   ├── override.go         # 一時 override 生成・削除
│   │   └── parse_test.go
│   ├── proxy/
│   │   ├── server.go           # リバースプロキシ
│   │   ├── handler.go          # HTTP / WebSocket ハンドラ
│   │   ├── daemon.go           # デーモン管理
│   │   └── server_test.go
│   ├── state/
│   │   ├── state.go            # state.json
│   │   ├── lock.go             # ファイルロック
│   │   ├── routes.go           # ルート登録・解除
│   │   └── state_test.go
│   ├── ports/
│   │   ├── allocator.go        # 空きポート割り当て
│   │   └── allocator_test.go
│   └── ui/
│       ├── output.go           # ターミナル出力
│       └── errors.go           # HTML エラーページ
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 16. ビルドと配布

```bash
GOOS=darwin GOARCH=arm64 go build -o dist/portless-docker-darwin-arm64 ./cmd/portless-docker
GOOS=darwin GOARCH=amd64 go build -o dist/portless-docker-darwin-amd64 ./cmd/portless-docker
GOOS=linux  GOARCH=amd64 go build -o dist/portless-docker-linux-amd64  ./cmd/portless-docker
GOOS=linux  GOARCH=arm64 go build -o dist/portless-docker-linux-arm64  ./cmd/portless-docker
```

| 方法 | コマンド |
|------|---------|
| GitHub Releases | バイナリダウンロード |
| Homebrew | `brew install portless-docker` |
| curl | `curl -fsSL https://get.portless-docker.dev \| sh` |
| go install | `go install github.com/xxx/portless-docker@latest` |

---

## 17. 将来の拡張

| 機能 | 概要 | 優先度 |
|------|------|--------|
| HTTPS | 自動証明書生成 + ローカル CA | 高 |
| HTTP/2 | allowHTTP1 フォールバック | 中 |
| docker-compose.yml 変更監視 | 変更検知で自動リロード | 中 |
| Docker Compose profiles | `--profile` 対応 | 低 |
| Web UI | ブラウザでルート確認 | 低 |
| 非 Docker サービス対応 | Docker 外プロセスの混在管理 | 低 |
