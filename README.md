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

redmine api get /issues/123.json
redmine api post /issues.json --body @issue.json
redmine api put /issues/123.json --body @issue.json
redmine api delete /issues/123.json
```

## Build

```bash
go build ./...
```
