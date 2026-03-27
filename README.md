# redmine-cli

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/nbifrye/redmine-cli)

Redmine REST API を操作するためのシンプルな CLI ツールです。

## インストール

### Homebrew

```bash
brew tap nbifrye/redmine-cli https://github.com/nbifrye/redmine-cli
brew install nbifrye/redmine-cli/redmine-cli
```

### ソースからビルド

```bash
go build ./...
```

## AI スキルのインストール

Claude Code などの AI エージェントから使う場合は、スキルをインストールしてください：

```bash
npx skills add nbifrye/redmine-cli
```

インストール後は `/redmine-cli` でスキルを呼び出せます。

## 使い方

詳しいコマンドリファレンスやワークフロー例は [`skills/redmine-cli/SKILL.md`](skills/redmine-cli/SKILL.md) を参照してください。

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

redmine api get /issues/123.json
redmine api post /issues.json --body @issue.json
redmine api put /issues/123.json --body @issue.json
redmine api delete /issues/123.json
```

グローバルフラグ: `--host`、`--api-key`、`--verbose`、`--debug`

## MCP サーバーとして使う（`redmine mcp serve`）

`redmine mcp serve` を実行すると、標準入出力（stdio）で動作する MCP サーバーとして `redmine` を公開できます。
MCP クライアント（Codex / Claude Code / Cursor など）から、issue / project / user の各サブコマンドをツールとして呼び出せるようになります。

```bash
redmine mcp serve
```

### 事前準備

- `redmine auth login` で認証情報を設定しておく
- または環境変数 `REDMINE_HOST` / `REDMINE_API_KEY` を設定する

### Codex で使う

Codex の MCP サーバー設定に、次のサーバー定義を追加します。

```json
{
  "mcpServers": {
    "redmine": {
      "command": "redmine",
      "args": ["mcp", "serve"],
      "env": {
        "REDMINE_HOST": "https://redmine.example.com",
        "REDMINE_API_KEY": "your-api-key"
      }
    }
  }
}
```

### Claude Code で使う

Claude Code の MCP 設定に同等のサーバー定義を追加するか、CLI から追加します。

```bash
claude mcp add redmine \
  --env REDMINE_HOST=https://redmine.example.com \
  --env REDMINE_API_KEY=your-api-key \
  -- redmine mcp serve
```

### Cursor で使う

Cursor の MCP 設定（例: `.cursor/mcp.json`）に、次のように追加します。

```json
{
  "mcpServers": {
    "redmine": {
      "command": "redmine",
      "args": ["mcp", "serve"],
      "env": {
        "REDMINE_HOST": "https://redmine.example.com",
        "REDMINE_API_KEY": "your-api-key"
      }
    }
  }
}
```

### 補足

- MCP 経由でも、通常 CLI と同じグローバルフラグ（`--host`、`--api-key`、`--verbose`、`--debug`）を利用できます。
- `issue.list` / `issue.view` / `project.list` / `user.view` など、既存サブコマンドが MCP ツールとして公開されます。
