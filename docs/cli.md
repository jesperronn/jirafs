# CLI

## Goal

The CLI is the primary interface for `jirafs`.

It must be usable without Codex or another agent layer.

## Command Families

Suggested top-level commands:

- `jirafs use`
- `jirafs mirror`
- `jirafs init`
- `jirafs export`
- `jirafs plan`
- `jirafs sync`
- `jirafs new`
- `jirafs registry`
- `jirafs board`
- `jirafs archive`

## Init

Creates the local project layout and config files.

Examples:

```text
jirafs init
jirafs init --jira-url https://jira.example.com
jirafs init --project PLAT --mirror-dir ~/jira/platform
```

## Use

Sets, clears, or shows the current Jira project context.

Examples:

```text
jirafs use
jirafs use platform
jirafs use --clear
```

## Mirror

Refreshes and manages named live mirror scopes.

Examples:

```text
jirafs mirror refresh current-sprint
jirafs mirror refresh my-issues
jirafs mirror refresh --all
jirafs mirror archive-sweep
jirafs mirror status
```

## Export

Exports remote Jira data into the local filesystem.

Examples:

```text
jirafs export issue ABC-123
jirafs export sprint current
jirafs export jql 'assignee = currentUser()'
jirafs export backlog
jirafs export selected ABC-123 OPS-9
```

## Plan

Shows the operations required to sync local changes back to Jira.

Examples:

```text
jirafs plan
jirafs plan jira/live/issues/ABC-123.md
```

## Sync

Applies the planned operations to Jira.

Examples:

```text
jirafs sync
jirafs sync jira/live/issues/ABC-123.md
jirafs sync --apply-transitions
```

## New

Creates a new draft issue from a template.

Examples:

```text
jirafs new story
jirafs new bug --summary 'Export fails on stale refs'
jirafs new epic --template jira/templates/epic.md
jirafs new story --project PLAT
```

## Registry

Refreshes or inspects reference registries.

Examples:

```text
jirafs registry refresh
jirafs registry show users
jirafs registry show sprints
```

## Board

Renders local issue projections such as sprint or kanban views.

Examples:

```text
jirafs board
jirafs board --sprint current
jirafs board --group-by assignee
jirafs board --group-by epic
jirafs board --project PLAT
```

## Archive

Moves or snapshots historical data into archive locations.

Examples:

```text
jirafs archive sprint platform-2026-w24
jirafs archive issue ABC-111
```

## Output Rules

The CLI should support:

- human-readable default output
- machine-readable JSON where useful
- explicit plan output for all write paths

## Config

Configuration should support:

- Jira base URL
- auth method
- multiple Jira instances
- project definitions
- mirror directories
- current-project memory
- local folder detection
- project defaults
- board ids
- custom field mappings
- section-to-field mappings

The current design direction is a global config file at
`~/.jirafs/settings.toml`, while still allowing environment or secret-manager
references for credentials.
