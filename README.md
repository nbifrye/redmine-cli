# redmine-cli

Redmine REST API を操作するためのシンプルな CLI ツールです。

## Install

### Homebrew

```bash
brew tap nbifrye/redmine-cli https://github.com/nbifrye/redmine-cli
brew install nbifrye/redmine-cli/redmine-cli
```

### Build from source

```bash
go build ./...
```

## AI スキルのインストール

Claude Code などの AI エージェントから使う場合は、スキルをインストールしてください：

```bash
npx skills add nbifrye/redmine-cli
```

インストール後は `/redmine-cli` でスキルを呼び出せます。

## Usage

```bash
redmine auth login
redmine auth status

redmine issue list --project myproj --status open
redmine issue view 123
redmine issue create --project myproj --subject "バグ修正"
redmine issue update 123 --status-id 2 --assigned-to-id 10
redmine issue close 123
redmine issue note-add 123 --notes "調査結果を追記"

redmine project list
redmine project view myproj
redmine project create --identifier myproj --name "My Project"

redmine user list --name taro
redmine user view 10

redmine time-entry list --user-id me --from 2025-01-01 --to 2025-01-31
redmine time-entry create --issue-id 123 --hours 1.5 --activity-id 9 --spent-on 2025-01-31

redmine api get /issues/123.json
redmine api post /issues.json --body @issue.json
redmine api put /issues/123.json --body @issue.json
redmine api delete /issues/123.json
```

グローバルフラグ: `--host`, `--api-key`, `--verbose`, `--debug`
