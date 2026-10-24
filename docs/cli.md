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
- `jirafs diff`
- `jirafs sync`
- `jirafs new`
- `jirafs registry`
- `jirafs board`
- `jirafs archive`

## Init

The single command that configures a project. It records one Jira instance and
one project in `~/.jirafs/settings.toml`, creates the mirror directory, and
writes `mirror.yml` plus the six registry files. Existing files are never
overwritten. There is no separate `setup` command.

The project name is the first positional argument; the Jira project key is
`--key`. Missing values are prompted for when run in a terminal.

Examples:

```text
jirafs init platform --key PLAT --instance work --base-url https://jira.example.com
jirafs init platform --key PLAT --instance work --base-url https://jira.example.com \
  --credential-ref env://JIRAFS_API_TOKEN --mirror-dir ~/jira/platform --set-current
```

## Flag conventions

- `--project NAME` always means the name of a project already configured in
  settings. It never means a Jira key (`init` uses `--key` for that, and takes
  the new project's name as its first positional argument).
- `--cwd DIR` sets the directory the project is resolved from. `status` and
  `doctor` accept both, like the other project-aware commands.
- `--base-url` is the Jira base URL everywhere.
- Results and data (listings, plans, exported content, paths) go to stdout.
  Progress, warnings, hints and errors go to stderr. Explicit `--help` prints
  to stdout.
- Usage errors exit 2, other failures exit 1.

## Use

Sets, clears, or shows the current Jira project context. Project listings are
sorted by name and printed to stdout.

Examples:

```text
jirafs use
jirafs use platform
jirafs use --project platform
jirafs use --clear
```

## Mirror

Refreshes and manages named live mirror scopes.

The primary live mirror scopes are:

- `current-sprint`
- `my-issues`
- `backlog`
- `next-sprint`

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

## Diff

Shows the operations required to sync local changes back to Jira. Read-only; only the remote issue is fetched. With no key it diffs every locally changed issue file. It prints `N issue(s) changed` and exits 0 whether or not there are differences; `--exit-code` exits 1 when any issue differs.

Examples:

```text
jirafs diff
jirafs diff ABC-123 ABC-124
jirafs diff --exit-code
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
