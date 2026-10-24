# CLI

## Goal

The CLI is the primary interface for `jirafs`.

It must be usable without Codex or another agent layer.

## Command Families

Suggested top-level commands:

- `jirafs use`
- `jirafs pull`
- `jirafs init`
- `jirafs diff`
- `jirafs push`
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

## Mirror Scopes

Named live mirror scopes, pulled with `jirafs pull <scope>`.

The primary live mirror scopes are:

- `current-sprint`
- `my-issues`
- `backlog`
- `next-sprint`

Examples:

```text
jirafs pull current-sprint
jirafs pull my-issues
jirafs pull
jirafs pull my-issues --full
jirafs archive sweep
```

## Pull

Fetches remote Jira data into the local filesystem. An argument shaped like an issue key (`ABC-123`) or a Jira browse URL pulls that one issue; any other argument is a scope name from `mirror.yml` (or a built-in scope such as `my-issues`), refreshed incrementally. With no argument every configured scope is pulled. A name that is both an issue key and a scope name is an error: rename the scope.

Examples:

```text
jirafs pull ABC-123
jirafs pull ABC-123 --stdout
jirafs pull my-issues
jirafs pull my-issues --full
jirafs pull --dry-run
```

## Diff

Shows the operations required to sync local changes back to Jira. Read-only; only the remote issue is fetched. With no key it diffs every locally changed issue file. It prints `N issue(s) changed` and exits 0 whether or not there are differences; `--exit-code` exits 1 when any issue differs.

Examples:

```text
jirafs diff
jirafs diff ABC-123 ABC-124
jirafs diff --exit-code
```

## Push

Writes local changes to Jira. A draft file (`state: draft`) is created as a new
issue and renamed to `KEY.md`; an existing issue (KEY or its file) is updated,
including status transitions. With no argument it pushes every locally modified
file and draft (as listed by `jirafs status`). Dry run unless `--apply`;
`--write-local` also refreshes the local file after an update. `--apply` with
`--dry-run` exits 2. Each issue is reported as `ok`, `skipped (conflict)` or
`failed`; one failure never stops the others and the exit code is 1 when any
issue failed. Conflicted files (remote changed since the last pull) are skipped.

Examples:

```text
jirafs push
jirafs push --apply
jirafs push ABC-123
jirafs push jira/live/issues/ABC-123.md --apply --write-local
jirafs push jira/add-dark-mode.md --apply
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
