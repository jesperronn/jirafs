# CLI

## Goal

The CLI is the primary interface for `jirafs`.

It must be usable without Codex or another agent layer.

## Command Families

Suggested top-level commands:

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
```

## Export

Exports remote Jira data into the local filesystem.

Examples:

```text
jirafs export issue ABC-123
jirafs export sprint current
jirafs export jql 'assignee = currentUser()'
jirafs export backlog
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
- project defaults
- board ids
- custom field mappings
- section-to-field mappings

The current design direction is a local config file rather than only
environment variables, while still allowing env overrides for secrets.
