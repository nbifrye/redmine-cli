---
name: redmine-cli
description: |
  Redmine インスタンスを操作する redmine-cli ツールを使う際に呼び出します。
  CLI を通じた認証、課題・プロジェクト・ユーザーの管理を支援します。
  /redmine-cli として呼び出すと Redmine ワークフローをサポートします。
user-invocable: true
argument-hint: "[タスクまたは質問]"
---

# redmine-cli スキル

あなたは `redmine-cli` ツールの専門アシスタントです。Redmine REST API を操作する Go 製 CLI です。

ユーザーから Redmine の操作を依頼されたら、適切な `redmine` コマンドを構築して実行してください。JSON 出力をそのまま表示するのではなく、内容を解釈して人間が読みやすい形式で結果を要約してください。

## セットアップ・認証

### 対話形式でログイン

```bash
redmine auth login
# プロンプトで入力: Redmine ホスト URL、次に API キー
# /users/current.json を呼び出して検証
```

```bash
redmine auth status   # 現在のホストと認証済みユーザーを表示
```

### 設定ファイル

場所: `~/.config/redmine-cli/config.yml`

```yaml
default_host: https://redmine.example.com
hosts:
  https://redmine.example.com:
    api_key: "your-api-key-here"
```

### 優先順位（高い順）

1. CLI フラグ（`--host`、`--api-key`）
2. 環境変数（`REDMINE_HOST`、`REDMINE_API_KEY`）
3. 設定ファイル（`~/.config/redmine-cli/config.yml`）

### CI / 非対話型での使用

```bash
export REDMINE_HOST=https://redmine.example.com
export REDMINE_API_KEY=your-api-key
redmine issue list --project myproj
```

---

## 課題ワークフロー

### 課題一覧

```bash
redmine issue list                                      # オープンな課題（デフォルト: --status open）
redmine issue list --project myproj                    # プロジェクトでフィルタ
redmine issue list --assigned-to me                    # 現在のユーザーに割り当て
redmine issue list --status "*"                        # すべてのステータス
redmine issue list --all                               # 最大 100 件の課題
redmine issue list --page 2 --per-page 50              # ページネーション
```

### 課題詳細の表示

```bash
redmine issue view 123                                  # JSON にジャーナル/ノートを含む
```

### 課題の作成

```bash
redmine issue create --project myproj --subject "ログインバグを修正"
redmine issue create --project myproj --subject "タスク" --description "詳細はこちら"
```

### 課題の更新

```bash
redmine issue update 123 --status-id 2
redmine issue update 123 --assigned-to-id 10 --subject "更新されたタイトル"
```

> ステータス ID が不明な場合は、次のコマンドで確認できます：
> ```bash
> redmine api get /issue_statuses.json
> ```

### 課題のクローズ

```bash
redmine issue close 123             # デフォルトはステータス ID 5 を使用
redmine issue close 123 --status-id 3   # ステータス ID を上書き
```

### 課題にノートを追加

```bash
redmine issue note-add 123 --notes "調査完了、コミット abc123 で修正済み"
```

---

## プロジェクトコマンド

```bash
redmine project list
redmine project view myproj              # 識別子または数値 ID で指定
redmine project create --identifier myproj --name "My Project"
redmine project create --identifier myproj --name "My Project" --description "説明"
```

> `--identifier` は一意かつ URL セーフ（小文字・数字・ハイフン）である必要があります。

---

## ユーザーコマンド

```bash
redmine user list                        # アクティブユーザー（デフォルト: --status 1）
redmine user list --name taro            # 名前で検索
redmine user list --status 3             # ロックされたユーザー
redmine user view 10                     # ID でユーザーを表示
redmine auth status                      # 現在の認証済みユーザーを表示
```

---

## 生 API エスケープハッチ

名前付きコマンドでカバーされていないエンドポイントに使用：

```bash
redmine api get /issue_statuses.json
redmine api get /issue_priorities.json
redmine api get /issues/123.json

redmine api post /issues.json --body '{"issue":{"project_id":"myproj","subject":"テスト"}}'
redmine api put /issues/123.json --body '{"issue":{"status_id":2}}'
redmine api delete /issues/123.json
```

---

## グローバルフラグ

| フラグ | 説明 |
|------|-------------|
| `--host <url>` | この呼び出しで使用する Redmine ホストを上書き |
| `--api-key <key>` | この呼び出しで使用する API キーを上書き |
| `--verbose` | リクエストのメソッド/URL とレスポンスステータスを stderr に表示 |
| `--debug` | HTTP ヘッダーとボディの全情報を stderr に表示 |

---

## エラーリファレンス

| 終了コード | 意味 |
|-----------|---------|
| 0 | 成功 |
| 1 | クライアントエラー（入力不正、401、404） |
| 2 | ネットワークまたはサーバーエラー |

よくあるエラーメッセージ：

- `authentication failed (401)` → `redmine auth login` を実行するか `REDMINE_API_KEY` を確認する
- `resource not found (404)` → プロジェクト識別子または課題 ID を確認する
- `host is not configured` → `redmine auth login` を実行するか `REDMINE_HOST` を設定する
- `API key is not configured` → `redmine auth login` を実行するか `REDMINE_API_KEY` を設定する
- `at least one field must be provided` → `issue update` に少なくとも 1 つのフラグを追加する
