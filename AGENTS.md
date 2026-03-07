# AGENTS.md

このリポジトリで作業する AI エージェント向けの運用ルールです。

## ディレクトリ構造と責務

```
redmine-cli/
├── main.go                    # エントリポイント。バージョン設定と cmd.Execute() のみ
├── cmd/
│   ├── root.go                # ルートコマンド定義・グローバルフラグ・サブコマンド登録
│   ├── runtime.go             # 設定読み込み・HTTPクライアント・エラーハンドリング
│   ├── auth.go                # auth login / status
│   ├── issue.go               # issue list / view / create / update / close / note-add
│   ├── project.go             # project list / view / create
│   ├── user.go                # user list / view
│   ├── time_entry.go          # time-entry list / create
│   └── api.go                 # 生API呼び出し (get / post / put / delete)
└── skills/
    └── redmine-cli/SKILL.md   # AI スキル定義（コマンドリファレンス）
```

## 設計方針

- **新しい Redmine リソースへの対応**は `cmd/<resource>.go` を追加し、`root.go` の `init()` で登録する。
- HTTP 通信はすべて `runtime.go` の `Runtime` 型を通じて行う（直接 `net/http` を呼ばない）。
- 出力は整形済み JSON のみ。フォーマット切り替えは現行スコープ外。

## テストと品質基準

- **テストカバレッジ 100% を維持**すること。
- バグ修正では、回帰テストを先に追加してから修正する。
- 失敗系（異常系・境界値）のテストを省略しない。

## テストコードの配置ルール

- テストファイルは実装ファイルと同じパッケージ・同じディレクトリに置く（例: `cmd/issue.go` → `cmd/issue_test.go`）。
- HTTP 通信のテストは `httptest.NewServer` でモックサーバーを立て、実ネットワークに依存しない。
- テスト用のヘルパー関数は、そのファイルのテスト内でのみ使うなら同ファイルに定義する。複数ファイルをまたぐ場合は `runtime_test.go` にまとめる。
- `exitFunc` はテスト内で差し替えて終了コードを検証する（`os.Exit` を直接呼ばない）。

## 実装ルール

- エラー文言・ログ文言は、運用時の検索性を下げない一貫した語彙を使う。
- エラーメッセージは「何が悪いか」と「次に何をすべきか」をセットで記述する（例: 401 → `auth login` を案内、404 → ID 確認を案内）。
- exit code を統一する: `0`=成功、`1`=想定内エラー（入力不正・404 等）、`2`=通信/内部エラー。

## PR 記載

PR では以下を必須記載とする：

- 背景 / 目的
- 主な変更点
- テスト観点（正常系・異常系）
- 互換性影響（あれば）

## Redmine API 仕様との整合性確認

Redmine のリソース操作を追加・変更する際は、必ず公式ドキュメントでパラメータ名を確認すること。

### 公式ドキュメントの参照方法

Redmine REST API ドキュメントは以下のパターンで参照できる：

- API 概要: `https://www.redmine.org/projects/redmine/wiki/Rest_api`
- 各リソース: `https://www.redmine.org/projects/redmine/wiki/Rest_<ResourceName>`
  - 例: Issues → `Rest_Issues`、Projects → `Rest_Projects`、Users → `Rest_Users`
  - 新しいリソースを追加する際も同じパターンでドキュメントを検索する

### よくある誤りパターン

| 誤り | 正しい | 理由 |
|---|---|---|
| `assigned_to` | `assigned_to_id` | アサイニーフィルタは `_id` サフィックスが必要 |
| `page=N` / `per_page=M` | `offset=N` / `limit=M` | Redmine REST API のページネーションは `offset`+`limit` 方式 |

### クエリパラメータの検証ルール

フィルタ・ページネーションのパラメータを追加・変更した際は、テストでモックサーバー側のクエリを必ず検証すること。テストがリクエスト内容を検証していない場合、誤ったパラメータ名を使っても気づけない。

```go
queryC := make(chan url.Values, 1)
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    queryC <- r.URL.Query()
    _, _ = w.Write([]byte(`{"resources":[]}`))
}))
// ...
got := <-queryC
if v := got.Get("offset"); v != "25" {
    t.Errorf("query param offset: got %q, want %q", v, "25")
}
```
