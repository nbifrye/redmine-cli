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
├── docs/
│   └── redmine-cli-design.md  # 設計方針・意思決定の記録
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

## 実装ルール

- エラー文言・ログ文言は、運用時の検索性を下げない一貫した語彙を使う。

## PR 記載

PR では以下を必須記載とする：

- 背景 / 目的
- 主な変更点
- テスト観点（正常系・異常系）
- 互換性影響（あれば）
