# redmine-cli

Redmine REST API を操作するためのシンプルな CLI です。`docs/redmine-cli-design.md` の実装方針（最小構成、JSON出力、フラグ優先度など）に基づいて MVP を実装しています。

## Documents

- [Redmine CLI 調査・設計メモ（GitHub CLI / GitLab CLI 参考）](docs/redmine-cli-design.md)

## Current MVP Commands

```bash
redmine auth login
redmine auth status

redmine issue list --project myproj --status open
redmine issue view 123
redmine issue create --project myproj --subject "バグ修正" --description "..."

redmine project list
redmine project view myproj

redmine api get /issues/123.json
redmine api post /issues.json --body @issue.json
```

## Build

```bash
go build ./...
```

## Homebrew (custom tap)

このリポジトリを tap として利用して `redmine` をインストールできます。

1. `Formula/redmine-cli.rb` 内の `YOUR_GITHUB_USER` を、実際の GitHub ユーザー/組織名に置き換える。
2. 変更を push した上で、以下を実行する。

```bash
brew tap YOUR_GITHUB_USER/redmine-cli
brew install redmine-cli
```

更新時は以下で最新版（main ブランチ）を再インストールできます。

```bash
brew reinstall redmine-cli
```
