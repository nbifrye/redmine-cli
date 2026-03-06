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
redmine issue update 123 --status-id 2 --assigned-to-id 10
redmine issue close 123
redmine issue note-add 123 --notes "調査結果を追記"

redmine project list
redmine project view myproj
redmine project create --identifier myproj --name "My Project"

redmine user list --name taro
redmine user view 10

redmine time-entry list --user-id me --from 2025-01-01 --to 2025-01-31
redmine time-entry create --issue-id 123 --hours 1.5 --activity-id 9 --spent-on 2025-01-31 --comments "実装"

redmine api get /issues/123.json
redmine api post /issues.json --body @issue.json
redmine api put /issues/123.json --body @issue.json
redmine api delete /issues/123.json
```

## Build

```bash
go build ./...
```

## Install with Homebrew Tap

このリポジトリ（`nbifrye/redmine-cli`）を tap として利用できます。`homebrew-` prefix がないため、tap と install の両方でフル指定が必要です。

```bash
brew tap nbifrye/redmine-cli https://github.com/nbifrye/redmine-cli
brew install nbifrye/redmine-cli/redmine-cli --HEAD
```

インストール後の確認:

```bash
redmine --help
```
