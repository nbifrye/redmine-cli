---
name: redmine-cli
description: |
  Use when working with the redmine-cli tool to interact with a Redmine instance.
  Helps authenticate, manage issues, projects, and users via CLI.
  Invoke as /redmine-cli to get assistance with Redmine workflows.
user-invocable: true
argument-hint: "[task or question]"
---

# redmine-cli Skill

You are an expert assistant for the `redmine-cli` tool — a Go-based CLI for the Redmine REST API.

When the user asks you to perform Redmine operations, construct and run the appropriate `redmine` commands. Interpret JSON output and summarize the results in human-readable form rather than dumping raw JSON.

## Setup & Authentication

### Interactive login

```bash
redmine auth login
# Prompts for: Redmine host URL, then API key
# Validates by calling /users/current.json
```

```bash
redmine auth status   # Show current host and authenticated user
```

### Configuration file

Location: `~/.config/redmine-cli/config.yml`

```yaml
default_host: https://redmine.example.com
hosts:
  https://redmine.example.com:
    api_key: "your-api-key-here"
```

### Priority order (highest to lowest)

1. CLI flags (`--host`, `--api-key`)
2. Environment variables (`REDMINE_HOST`, `REDMINE_API_KEY`)
3. Config file (`~/.config/redmine-cli/config.yml`)

### CI / non-interactive usage

```bash
export REDMINE_HOST=https://redmine.example.com
export REDMINE_API_KEY=your-api-key
redmine issue list --project myproj
```

---

## Issue Workflows

### List issues

```bash
redmine issue list                                      # Open issues (default: --status open)
redmine issue list --project myproj                    # Filter by project
redmine issue list --assigned-to me                    # Assigned to current user
redmine issue list --status "*"                        # All statuses
redmine issue list --all                               # Up to 100 issues
redmine issue list --page 2 --per-page 50              # Pagination
```

### View issue detail

```bash
redmine issue view 123                                  # Includes journals/notes in JSON
```

### Create issue

```bash
redmine issue create --project myproj --subject "Fix login bug"
redmine issue create --project myproj --subject "Task" --description "Details here"
```

### Update issue

```bash
redmine issue update 123 --status-id 2
redmine issue update 123 --assigned-to-id 10 --subject "Updated title"
```

> If status IDs are unknown, discover them:
> ```bash
> redmine api get /issue_statuses.json
> ```

### Close issue

```bash
redmine issue close 123             # Uses status ID 5 by default
redmine issue close 123 --status-id 3   # Override status ID
```

### Add note to issue

```bash
redmine issue note-add 123 --notes "Investigation complete, fixed in commit abc123"
```

---

## Project Commands

```bash
redmine project list
redmine project view myproj              # By identifier or numeric ID
redmine project create --identifier myproj --name "My Project"
redmine project create --identifier myproj --name "My Project" --description "Description"
```

> `--identifier` must be unique and URL-safe (lowercase letters, numbers, hyphens).

---

## User Commands

```bash
redmine user list                        # Active users (default: --status 1)
redmine user list --name taro            # Search by name
redmine user list --status 3             # Locked users
redmine user view 10                     # View user by ID
redmine auth status                      # View current authenticated user
```

---

## Raw API Escape Hatch

For endpoints not covered by named commands:

```bash
redmine api get /issue_statuses.json
redmine api get /issue_priorities.json
redmine api get /issues/123.json

redmine api post /issues.json --body '{"issue":{"project_id":"myproj","subject":"Test"}}'
redmine api put /issues/123.json --body '{"issue":{"status_id":2}}'
redmine api delete /issues/123.json
```

---

## Global Flags

| Flag | Description |
|------|-------------|
| `--host <url>` | Override configured Redmine host for this invocation |
| `--api-key <key>` | Override configured API key for this invocation |
| `--verbose` | Print request method/URL and response status to stderr |
| `--debug` | Print full HTTP headers and body to stderr |

---

## Error Reference

| Exit code | Meaning |
|-----------|---------|
| 0 | Success |
| 1 | Client error (bad input, 401, 404) |
| 2 | Network or server error |

Common error messages:

- `authentication failed (401)` → Run `redmine auth login` or check `REDMINE_API_KEY`
- `resource not found (404)` → Verify the project identifier or issue ID
- `host is not configured` → Run `redmine auth login` or set `REDMINE_HOST`
- `API key is not configured` → Run `redmine auth login` or set `REDMINE_API_KEY`
- `at least one field must be provided` → Add at least one flag to `issue update`
