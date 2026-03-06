# Redmine CLI 調査・設計メモ（GitHub CLI / GitLab CLI 参考）

## 目的

Redmine REST API を扱う CLI ツールを設計する。参考実装として `gh`（GitHub CLI）と `glab`（GitLab CLI）を調査し、実装方針を整理する。特に以下を対象とする。

- 設計思想
- コマンド/インターフェース設計
- オプション設計
- 認証フロー
- 技術選定
- 配布・運用
- 最終的な Redmine CLI 実装方針

> 方針上の制約: パッケージ分割や過度なレイヤー化を避け、愚直でシンプルなアーキテクチャを維持する。

---

## 1. 参考実装サマリ

### 1.1 GitHub CLI (`gh`) から得られる知見

- **UX 優先のコマンド設計**
  - リソース名 + 動詞の構造（例: `pr create`, `issue list`）。
  - `--json`, `--jq`, `--template` による機械可読 + 人間可読の両立。
- **認証導線が強い**
  - `auth login` による対話型セットアップ。
  - PAT / デバイスフロー / SSH 鍵設定など用途に応じた分岐。
- **設定のわかりやすい優先順位**
  - フラグ > 環境変数 > 設定ファイル > デフォルト。
- **API 直叩きの逃げ道**
  - `gh api` のような汎用エンドポイント実行手段がある。
- **拡張性**
  - 拡張コマンド機構（`gh extension`）。

### 1.2 GitLab CLI (`glab`) から得られる知見

- **GitLab ワークフロー密着**
  - MR、Issue、Pipeline などを日常操作と一致する形で提供。
- **ホスト/インスタンス前提**
  - `gitlab.com` だけでなくセルフホスト前提でホスト切替を重視。
- **認証トークン管理**
  - PAT を中心に、ホストごとに認証情報を保持。
- **CI/CD 連携の実用性**
  - スクリプト利用を意識した非対話モード設計。

### 1.3 `gh` / `glab` 共通の設計学び

1. 初回セットアップ（認証 + ホスト設定）を**最短導線**で行える。
2. サブコマンド体系は**ドメイン語彙**と一致させる。
3. 出力は「人間向け（表）」と「機械向け（JSON）」を両立。
4. フラグ体系は全コマンドで**一貫性**を持たせる。
5. 例外系（レート制限、401、404）を明確に表示。
6. 非対話利用（CI）を最初から想定する。

---

## 2. Redmine CLI への落とし込み（要求別）

## 2.1 対象ユースケース

最初のスコープは以下に絞る。

- Issue の一覧/詳細/作成/更新
- Project の一覧/詳細
- 自分に割り当てられたチケット確認
- API 疎通確認 (`whoami`, `version` 的コマンド)
- 汎用 API 実行（将来の拡張を見据えて）

## 2.2 コマンドインターフェース案

`gh` / `glab` の「リソース + 動詞」を採用。

```bash
redmine auth login
redmine auth status
redmine config set host https://redmine.example.com

redmine issue list --project myproj --status open
redmine issue view 123
redmine issue create --project myproj --subject "バグ修正" --description "..."
redmine issue update 123 --status "In Progress" --assigned-to me

redmine project list
redmine project view myproj

redmine api get /issues/123.json
redmine api post /issues.json --body @issue.json
```

### グローバルオプション案

- `--host <url>`: 接続先 Redmine（設定を一時上書き）
- `--api-key <token>`: API キー（設定を一時上書き、CI向け）
- `--verbose`: リクエスト/レスポンス概要表示
- `--debug`: HTTP 詳細（ヘッダ等）
- `--no-color`: 色抑止

> 出力は常に JSON（デフォルト）とし、出力フォーマット切替オプションは持たない。

## 2.3 オプション設計原則

- 省略形は**よく使うもののみ**（`-p`, `-s` など最小限）。
- フィルタは API パラメータと対応づける。
- ページングは共通化。
  - `--page`, `--per-page`, `--all`
- JSON入力を受ける場合は `--body @file.json` を許可。

---

## 3. 認証・設定設計

## 3.1 認証方式

Redmine では API キー利用が中心のため、まずは API キーを主軸にする。

- `auth login`（対話）
  1. Host 入力
  2. API キー入力（非表示）
  3. `/users/current.json` で検証
  4. 設定保存

- 非対話（CI）
  - `REDMINE_HOST`
  - `REDMINE_API_KEY`

## 3.2 設定の優先順位

`gh`/`glab` と同様に以下で固定する。

1. CLI フラグ
2. 環境変数
3. 設定ファイル
4. デフォルト

## 3.3 設定ファイル

例: `~/.config/redmine-cli/config.yml`

```yaml
default_host: https://redmine.example.com
hosts:
  https://redmine.example.com:
    api_key: "***"
```

> API キーは将来的に OS キーチェーン保管へ移行余地を残す。初期実装では設定ファイル格納でも可（権限 600 を強制）。

---

## 4. 技術要素（実装スタック）

## 4.1 言語

Go を推奨。

- 単一バイナリ配布がしやすい。
- `gh` / `glab` も Go 中心で、CLI 開発の知見が多い。
- クロスコンパイルと配布自動化が容易。

## 4.2 利用ライブラリ（最小）

- CLI パーサ: `cobra`
- 設定: `viper`（または標準ライブラリ + YAML のみ）
- HTTP: `net/http`（追加依存を抑える）
- 出力整形: `encoding/json`

> 依存を増やしすぎず、`gh` のような高機能化は段階的に行う。

---

## 5. シンプル実装アーキテクチャ（重要）

過度な分割を避けるため、初期実装は次の「単純な3層未満」を維持する。

## 5.1 ディレクトリ案（最小）

```text
.
├─ main.go
├─ go.mod
├─ cmd/
│  ├─ root.go
│  ├─ auth.go
│  ├─ issue.go
│  ├─ project.go
│  └─ api.go
```

- `cmd/*`: CLI 定義と引数解釈（薄く保つ）
- `main.go`: 設定読み込み + HTTP 呼び出し + 主要ロジックを集約

> `internal/app.go` への分離すら最初は行わず、`main.go` に寄せる。いわゆる `service/repository/domain` 分割は**初期段階では採用しない**。

## 5.2 処理フロー

1. `root` で設定とフラグを確定
2. `main.go` 内の共通関数に `host`, `apiKey`, `verbose` を渡す
3. 同共通関数が HTTP を実行
4. 結果をコマンド側で表示

この方式なら可読性が高く、個人/小規模チームで保守しやすい。

---

## 6. エラーハンドリングと UX 方針

- エラーは「何が悪いか / 次に何をすべきか」をセットで出す。
  - 401: APIキー不正（`redmine auth login` を案内）
  - 404: project/issue ID を確認するよう案内
- `--verbose` でリクエストIDや URL を表示可能にする。
- exit code を統一する。
  - 0: success
  - 1: 想定内エラー（入力不正、404等）
  - 2: 通信/内部エラー

---

## 7. 配布・運用

## 7.1 配布

- GitHub Releases で各 OS 向けバイナリ配布
- Homebrew Tap 提供
- 将来: scoop / winget / apt リポジトリ

## 7.2 バージョニング

- SemVer (`v0.x` で高速改善 → `v1.0` で安定化)

## 7.3 CI

- `go test ./...`
- `golangci-lint`
- `goreleaser` で自動ビルド・署名・配布

---

## 8. 実装ロードマップ

## Phase 1（MVP）

- `auth login`, `auth status`
- `issue list`, `issue view`, `issue create`
- `project list`, `project view`
- JSON 出力（デフォルト）

## Phase 2

- `issue update`, `issue close`, `issue note add`
- `api` 汎用コマンド
- ページング強化（`--all`）

## Phase 3

- 出力テンプレート
- インタラクティブ選択（fzf 的補助）
- 拡張機構（サブコマンド外部実行）

---

## 9. 最終提案（今回の結論）

1. **CLI 体験は `gh`/`glab` に合わせて「リソース + 動詞」で統一する。**
2. **認証は API キー中心、対話 + 非対話の両方を最初から提供する。**
3. **設定優先順位（flag > env > file）を明文化して固定する。**
4. **実装はシンプルに保ち、まずは `main.go` 集約 + `cmd` の最小構成で始める。**
5. **MVP は issue/project/auth に限定し、先に安定運用できる CLI を目指す。**

この方針なら、複雑な抽象化を避けつつ、将来の拡張（時間管理・Wiki・チケット関係性操作）にも十分対応できる。
