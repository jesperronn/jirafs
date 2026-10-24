# CLI

## Goal

The CLI is the primary interface for `jirafs`.

It must be usable without Codex or another agent layer.

## Command Families

<!-- BEGIN GENERATED: commands -->
| Command | Description |
| --- | --- |
| `jirafs init` | configure a Jira instance and project, and create its mirror directory |
| `jirafs pull` | fetch Jira issues into local Markdown files (one issue or a scope) |
| `jirafs diff` | show what push would change in Jira (read-only) |
| `jirafs status` | show workspace status and next-step hints |
| `jirafs doctor` | run health checks on config, credentials, and live connectivity |
| `jirafs new` | create a new issue from a template |
| `jirafs push` | create drafts and update modified issues in Jira (dry run unless --apply) |
| `jirafs registry` | manage local registry files for typed references |
| `jirafs board` | show a local kanban-style board view |
| `jirafs archive` | archive resolved issues and list archived files |
| `jirafs archive issue` | archive a single resolved issue by key |
| `jirafs archive sweep` | report (or with --apply, archive) archive-eligible issues |
| `jirafs archive list` | list archived issue snapshots |
| `jirafs use` | update remembered project context |
| `jirafs help` | show this help message |
<!-- END GENERATED -->

Per-command usage and flags, generated from the command registry in
`internal/cli/commands.go` (run `bin/gendocs` after changing it):

<!-- BEGIN GENERATED: command-reference -->
### `jirafs init`

configure a Jira instance and project, and create its mirror directory

```text
jirafs init [flags] <project-name>
```

Configures one Jira instance and one project: records them in ~/.jirafs/settings.toml, creates the mirror directory and writes mirror.yml plus the six registry files (users, projects, statuses, sprints, fix_versions, issue_types). Existing files are left untouched. Missing values are prompted for in a terminal.

Flags:

- `--auth-type string`: auth type: basic, atlassian_api_token, bearer_token or oauth1 (default: atlassian_api_token)
- `--base-url string`: Jira base URL (http:// or https://)
- `--credential-ref value`: credential reference (env://..., file://... or op://...); may be repeated
- `--instance string`: settings name for the Jira instance
- `--key string`: Jira project key (e.g. PLAT)
- `--mirror-dir string`: mirror directory (default: ~/jira/<project>)
- `--set-current`: also set the remembered current project
- `--help, -h`: show this help message

### `jirafs pull`

fetch Jira issues into local Markdown files (one issue or a scope)

```text
jirafs pull [scope|KEY] [flags]
```

Fetches issues from Jira into the local mirror. KEY (PROJ-123, or a Jira URL) pulls that one issue to <local_dirs[0]>/<KEY>.md. A scope name from mirror.yml (or the built-ins my-issues and current-sprint) is refreshed incrementally; files with local edits are kept. With no argument, every scope configured in mirror.yml is pulled.

Examples:

- `jirafs pull`: pull every configured scope
- `jirafs pull my-issues`: refresh one scope
- `jirafs pull PLAT-123`: pull one issue
- `jirafs pull PLAT-123 --stdout`: print one issue instead of writing a file

Flags:

- `--project NAME`: name of a configured project (default: resolved from --cwd or the remembered project)
- `--cwd DIR`: working directory for project resolution
- `--full`: scope only: ignore the last-sync cursor and refetch every issue
- `--stdout`: KEY only: print the document and write no file
- `--dry-run`: print what would be fetched and the real command; write nothing
- `--help, -h`: show this help message

### `jirafs diff`

show what push would change in Jira (read-only)

```text
jirafs diff [issue-key...] [flags]
```

Fetches each remote issue from Jira, reads the local copy from the file system, and prints the operations needed to bring the remote in line with the local copy. Without a key, diffs every locally changed issue. Read-only: nothing is written to Jira or to local files.

Flags:

- `--project NAME`: name of a configured project
- `--cwd DIR`: working directory for project resolution
- `--exit-code`: exit 1 when any issue differs (default exit 0)
- `--help, -h`: show this help message

### `jirafs status`

show workspace status and next-step hints

```text
jirafs status [flags]
```

Reports the current state of a jirafs workspace: config, mirror files, scopes, local changes (modified, draft, conflicted; offline), and a next-step hint for onboarding.

Flags:

- `--project NAME`: name of a configured project (default: resolved from --cwd or the remembered project)
- `--cwd DIR`: working directory to resolve the project from (default: current directory)
- `--porcelain`: print STATE<TAB>KEY<TAB>PATH lines only, for scripts
- `--help, -h`: show this help message

### `jirafs doctor`

run health checks on config, credentials, and live connectivity

```text
jirafs doctor [flags]
```

Reports the health of a jirafs workspace: config, credential resolution, and live-probe connectivity for each configured Jira instance.

Flags:

- `--project NAME`: name of a configured project (default: resolved from --cwd or the remembered project)
- `--cwd DIR`: working directory to resolve the project from (default: current directory)
- `--verbose`: print request URLs during checks
- `--help, -h`: show this help message

### `jirafs new`

create a new issue from a template

```text
jirafs new <story|bug|epic> [flags]
jirafs new --template <path.md> [flags]
jirafs new <story|bug|epic> --prompt [--idea <text>]
jirafs new --from <file|-> [flags]
```

Writes a new draft issue file to the project's local directory. It does not contact Jira. Built-in templates: story, bug, epic.

Flags:

- `--summary <text>`: set the issue summary
- `--project <name>`: name of a configured project
- `--assignee <name>`: set the assignee
- `--cwd <dir>`: working directory to resolve the project from
- `--template <path>`: use a user-supplied template file instead of a built-in
- `--prompt`: print the template plus instructions for an AI; no draft is written
- `--idea <text>`: with --prompt: the idea the AI should turn into the issue
- `--force`: overwrite the draft file if it already exists
- `--from <file|->`: create a draft from a filled document (- reads stdin)
- `--help, -h`: show this help message

AI pipe:

- `jirafs new story --prompt --idea "..." | llm | jirafs new --from -`

### `jirafs push`

create drafts and update modified issues in Jira (dry run unless --apply)

```text
jirafs push [KEY|file...] [flags]
```

Writes local issue files to Jira. A draft file (state: draft) is created as a new issue and rewritten as KEY.md; an existing issue (a KEY, or its file) is updated, including a status change through Jira's transitions. Without arguments it pushes every locally modified file and every draft, as listed by `jirafs status`. Files whose remote copy changed since the last pull are skipped and reported as conflicts. Without --apply (or with --dry-run) it only prints what would be written. Every issue is reported as ok, skipped or failed; one failure never stops the others, and the exit code is 1 when any issue failed. With git: true in mirror.yml, --apply commits what Jira accepted as "push to <host> at <time>".

Flags:

- `--apply`: perform the Jira writes (without it, push is a dry run)
- `--dry-run`: print what would be written and write nothing (the default)
- `--write-local`: after a successful update, write the refreshed remote to the local file
- `--project NAME`: name of a configured project
- `--cwd DIR`: working directory to resolve the project from
- `--help, -h`: show this help message

### `jirafs registry`

manage local registry files for typed references

```text
jirafs registry [flags] <subcommand> [args]
```

Manage local typed-reference registry files (users, projects, statuses, sprints, fix_versions). These live in the active project's mirror directory.

Subcommands:

- `show <type>`: display a registry; add --json for JSON output
- `init <type>`: create an empty registry file
- `refresh <type>`: populate a registry from Jira (requires network access)

Types:

- `users, projects, statuses, sprints, fix_versions`

Flags:

- `--project`: name of a configured project
- `--cwd`: working directory for project resolution
- `--json`: output registries as JSON (show)
- `--help, -h`: show this help message

### `jirafs board`

show a local kanban-style board view

```text
jirafs board [flags]
```

Displays a local kanban-style board view of issues grouped by status, assignee, or epic.

Flags:

- `--group-by MODE`: status, assignee, or epic (default: status)
- `--project KEY`: name of a configured project
- `--cwd DIR`: working directory for project resolution
- `--help, -h`: show this help message

### `jirafs archive`

archive resolved issues and list archived files

```text
jirafs archive <subcommand> [flags]
```

Moves resolved issues out of the live workspace into a per-project _archive directory and lists what has been archived.

Subcommands:

- `issue`: archive a single issue by key
- `sweep`: report (or with --apply, archive) archive-eligible issues
- `list`: list archived issue snapshots

Flags:

- `--project`: name of a configured project
- `--cwd`: working directory to resolve the project from
- `--help, -h`: show this help message

#### `jirafs archive issue`

archive a single resolved issue by key

```text
jirafs archive issue <KEY> [flags]
```

Moves one resolved issue file into the project's _archive directory.

Flags:

- `--project KEY`: name of a configured project
- `--cwd DIR`: working directory to resolve the project from
- `--dry-run`: print what would be archived; move nothing
- `--help, -h`: show this help message

#### `jirafs archive sweep`

report (or with --apply, archive) archive-eligible issues

```text
jirafs archive sweep [flags]
```

Reports archive-eligible issues in the project's local directories; with --apply, archives them.

Flags:

- `--project KEY`: name of a configured project
- `--cwd DIR`: working directory for project resolution
- `--apply`: actually archive eligible issues
- `--dry-run`: list eligible issues only (the default)
- `--help, -h`: show this help message

#### `jirafs archive list`

list archived issue snapshots

```text
jirafs archive list [flags]
```

Lists archived issue snapshots in the project's _archive directory.

Flags:

- `--project KEY`: name of a configured project
- `--cwd DIR`: working directory to resolve the project from
- `--help, -h`: show this help message

### `jirafs use`

update remembered project context

```text
jirafs use [flags] [project]
```

Shows, sets or clears the remembered current project. With no arguments it shows the current project.

Flags:

- `--clear`: clear the remembered current project
- `--project string`: name of the configured project to set as current
- `--help, -h`: show this help message

### `jirafs help`

show this help message

```text
jirafs help
```

Lists all commands. Run "jirafs <command> --help" for one command.
<!-- END GENERATED -->

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
