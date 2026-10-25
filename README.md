# jirafs

`jirafs` is a local-first Jira workspace.

The project goal is to make Jira issues editable as structured Markdown files,
sync those files safely back to Jira, and retain a durable archive that can be
used for analysis, agent workflows, and team process improvement.

## Current Workflow

The current CLI is usable as a narrow operator workflow:

1. Configure one Jira instance and one project with `jirafs init`.
2. Select or confirm the active project with `jirafs use`.
3. Pull one live mirror scope with `jirafs pull <scope>` (or every configured scope with `jirafs pull`).
4. Pull or inspect one issue locally with `jirafs pull KEY`.
5. Edit the mirrored Markdown file.
6. Review the planned write-back with `jirafs diff`.
7. Apply the write path with `jirafs push KEY --apply` (plain `jirafs push KEY` is a dry run).

Example:

```bash
jirafs init platform \
  --key PLAT \
  --instance work \
  --base-url https://jira.example.com \
  --auth-type atlassian_api_token \
  --credential-ref file://~/.jirafs/credentials/work-user.toml \
  --credential-ref env://JIRAFS_WORK_API_TOKEN \
  --set-current

jirafs use
jirafs pull my-issues
jirafs pull PLAT-123                    # writes <local_dir>/PLAT-123.md
jirafs pull PLAT-123 --stdout > /tmp/PLAT-123.md
jirafs diff PLAT-123
jirafs push PLAT-123                    # dry run: prints the plan, writes nothing
jirafs push PLAT-123 --apply            # writes to Jira
jirafs push PLAT-123 --apply --write-local   # ...and refreshes the local file
jirafs push                             # dry run for every modified file and draft
jirafs push --apply                     # ...and write them all
```

`jirafs push [KEY|file...]` is the only command that writes to Jira. A draft
file (`state: draft`) is created as a new issue; an existing issue (a KEY or its
file) is updated, including status transitions. It is a dry run unless you pass
`--apply`; `--write-local` (only with `--apply`) also writes the refreshed
remote to the local file. `--apply` with `--dry-run` is rejected (exit 2). Each
dry run ends with the exact command to run it for real.

With no arguments `push` handles every file `jirafs status` lists as modified
or draft. It prints one line per issue and keeps going when one fails:

```text
TEST-4  skipped (conflict)  remote changed since the last pull; review it: jirafs diff TEST-4
TEST-2  ok  applied 2 operation(s)
TEST-1  ok  created from jira/add-dark-mode.md
```

A conflicted file (remote changed since its last pull, or a `conflicts:` block
recorded by `pull`) is skipped and reported, never overwritten. A file with a
recorded `conflicts:` block is refused whatever its timestamps say; when it is
named explicitly with `--apply` the exit code is 1. The exit code is also 1
when any issue failed.

## Mirror scopes

Scopes live in `<mirror_dir>/mirror.yml`. Pull one by name with
`jirafs pull <scope-name>`; `jirafs pull` with no argument pulls every scope in
`mirror.yml`. An argument shaped like an issue key (`PLAT-123`) pulls that one
issue, so a scope must not be named like an issue key.

Refresh is incremental. After a refresh that finishes without failures, the
scope records `last_synced` (RFC3339) in `mirror.yml`. The next refresh asks
Jira only for `(<scope jql>) AND updated >= "yyyy/MM/dd HH:mm"` (the cursor
minus a 12 hour one minute overlap, formatted in UTC; a trailing `ORDER BY`
stays outside the wrapper). Jira reads that date in the Jira user's profile
timezone (UTC-12 to UTC+14), which jirafs cannot know, so the overlap makes the
window start no later than the real cursor in any zone; issues re-fetched
inside the overlap are reported as `unchanged` and not rewritten) and prints `fetched N, updated M, unchanged K`. Issue files whose
`updated` already matches Jira are not rewritten, and files with unsynced local
edits are never overwritten (they are listed with a `jirafs status` hint).
`--full` ignores the cursor and refetches the whole scope; `--dry-run` never
moves the cursor; a refresh with failures leaves it unchanged.

```yaml
scopes:
  - name: plat
    type: project        # project = "PLAT" ORDER BY key
    target: PLAT
  - name: urgent
    type: jql            # target is sent to Jira verbatim
    target: project = PLAT AND priority = Highest ORDER BY created
  - name: plat-api
    type: component      # needs "project"; project = "PLAT" AND component = "API"
    project: PLAT
    target: API
```

`my-issues` and `current-sprint` also work without a `mirror.yml` entry.
Unknown scope names print the configured names and an example to add.

## Base snapshots

After every successful `pull` or `push` write of an issue, jirafs stores the
rendered remote-side content of that issue (without `synced_hash` and
`sync_time`) at `.jirafs/base/KEY.md` under the mirror directory. It is the
merge base for per-field and three-way comparison. The `.jirafs/` directory is
jirafs-internal state and is not to be edited by hand; a missing or unreadable
base is treated as "no base" and never blocks a pull.

## Deleted and moved issues

`jirafs pull KEY` and `jirafs pull --full SCOPE` never delete a file. An issue Jira no longer has is marked `remote_state: gone` and moved into `_archive`; a moved issue is written under its new key and the old file is archived with `remote_state: moved` and `moved_to`. Local edits stay in the archived file, and `push` refuses archived files. Each case prints one line, for example:

```
gone PLAT-9: moved to _archive/PLAT-9.md (restore: mv _archive/PLAT-9.md .)
moved PLAT-9 -> PLAT-12: jirafs pull PLAT-12
```

`--dry-run` prints the same lines prefixed `would`.

## Merging local edits on `pull`

A file with unsynced local edits is never overwritten by `pull`. When the
remote changed since the last pull, jirafs merges it against the base snapshot
field by field: a field changed only remotely is taken, a field changed only
locally is kept, and a description edited in both places is merged by line.
`pull` prints one line per issue:

```text
merged PLAT-1 (backup: .jirafs/backup/PLAT-1-20260102T103000Z.md)
conflict PLAT-2 summary (backup: ...; review: jirafs diff PLAT-2)
```

A field changed on both sides to different values (or overlapping description
lines) is conflicted: the file keeps the local value, the remote value goes
into the frontmatter as `conflicts: [{field, remote, base}]`, and the base is
not advanced for that field so the next pull reproduces the conflict. Without a
base snapshot every differing field is conflicted. The merged file still counts
as locally modified. Before any merge or conflict the local file is saved to
`.jirafs/backup/KEY-<timestamp>.md`, so no local edit is lost. If the remote has
not changed since the last pull, the edited file is left as it is.

### Resolving conflicts

`jirafs status` lists every file with a `conflicts:` block as `conflicted`,
with its fields, `jirafs diff KEY` to review it and the resolve command;
`jirafs status --porcelain` prints `conflicted` lines. `push` refuses such
files until they are resolved:

```text
jirafs resolve PLAT-2 --keep local            # keep my values, drop the remote ones
jirafs resolve PLAT-2 --keep remote           # take the remote values (local saved first)
jirafs resolve PLAT-2 --keep remote --field summary   # only one field (comma-separated)
```

`--keep local` keeps the local value and advances the base snapshot for the
field to the remote value, so the next `push` sends the local value on
purpose. `--keep remote` first saves the local file to `.jirafs/backup/`,
then takes the recorded remote value. Resolved entries are removed from
`conflicts:`; the other fields stay conflicted, and removing the last entry
clears the key. Like every write command it is a dry run until `--apply`. In a
git-backed mirror `resolve` does not commit: it edits a local file you are still
working on, exactly like editing it by hand, and `push --apply` commits it once
it reached Jira.

Editing the file by hand and deleting the `conflicts:` block is also accepted;
`status` and `push` then treat it as a normal modified file. Prefer `resolve`:
it also advances the base snapshot, which a hand edit does not, so after a hand
edit `push` may still report the same fields as changed on both sides.

## Current Status

What works today:

- user-level setup writes `~/.jirafs/settings.toml`
- project selection and remembered current-project state
- pull wiring for named scopes such as `my-issues`
- export, plan, and sync command paths for the current safe field set
- `jirafs status` lists local changes offline, grouped as `conflicted`,
  `modified` and `draft`, each with a copy-pasteable command;
  `jirafs status --porcelain` prints `STATE<TAB>KEY<TAB>PATH` lines. Export,
  refresh, sync and push stamp each issue file with a `synced_hash`
  frontmatter field (a SHA-256 of the file minus `sync_time` and
  `synced_hash`) and record Jira's `updated` timestamp under `remote_updated`
  in `mirror.yml`. A file is `modified` when its hash differs from
  `synced_hash`, and `conflicted` when it is also older than `remote_updated`.
  Files without a `synced_hash` are not reported.
- credential refs from `env://`, `file://`, and `op://`
- auth support for `basic`, `atlassian_api_token`, and `bearer_token`

What is still rough:

- the README has historically assumed the deeper docs carry most of the
  operator story
- troubleshooting output should keep "useful next step" hints as concrete
  copy-pasteable commands, not just descriptive text
- live Jira behavior still needs real-instance shakeout and better diagnostics

## Obvious Next Steps

If you are trying `jirafs` for the first time:

1. Run `jirafs init <name> ... --set-current`.
2. Run `jirafs use` to confirm the active project.
3. Run `jirafs pull my-issues`.
4. Inspect the mirror directory and `mirror.yml`.
5. Run `jirafs pull ABC-123` or open a mirrored issue file.
6. Run `jirafs diff ABC-123` before any write attempt.

If something fails:

- no settings yet (first run): run
  `jirafs init <name> --key <KEY> --instance <name> --base-url https://<site>.atlassian.net --credential-ref env://JIRAFS_API_TOKEN --set-current`
- config or auth issue: inspect `~/.jirafs/settings.toml`
- scope issue: inspect `<mirror_dir>/mirror.yml`
- project resolution issue: run `jirafs use <name>`
- live Jira issue: retry with `jirafs pull KEY` for a single issue before a whole-scope pull
- empty sync scope: run `jirafs pull current-sprint` or
  `jirafs pull my-issues`, then `jirafs push` to write your edits back
- not sure what a command will change: add `--dry-run` (push, pull,
  new, archive issue, archive sweep); it prints
  the plan and the command for the real run, and writes nothing

## Testing Policy

Tests and agent verification must stay offline unless a task explicitly says
otherwise.

- mock 1Password and Jira calls by default
- use fake transports or injected providers for network-bound code
- do not add live endpoint tests as a shortcut for missing test seams
- if a live probe is genuinely required, state that in the task and the test
  name

## Live testing

`bin/live-throwaway [KEY]` runs the pull, edit, diff, push, pull and restore
lifecycle against one existing throwaway story in your **live** Jira (default
`SKOLELOGIN-18655`, or `$JIRAFS_THROWAWAY_KEY`). It touches only that issue,
asks for confirmation before the first `push --apply` (`--yes` skips it), and
restores the original values; step results are PASS, FAIL, UNSUPPORTED or
SKIPPED. Run `bin/live-throwaway --help` for options (`--fix-version`,
`--custom-field`, `--priority`, `--assignee`) and exit codes. It is manual
only and is not part of `bin/verify`.

## TLS Certificates

`jirafs` always verifies Jira server certificates against the system trust
store; there is no option to skip verification. For a Jira instance signed by a
private CA, add that CA certificate to the operating system trust store (for
example `update-ca-certificates` on Debian/Ubuntu, or Keychain Access on
macOS) and `jirafs` will trust it.

## Command Surface

Run `jirafs <command> --help` for flags. The table below is generated from the
command registry; edit `internal/cli/commands.go` and run `bin/gendocs`.

<!-- BEGIN GENERATED: commands -->
| Command | Description |
| --- | --- |
| `jirafs init` | configure a Jira instance and project, and create its mirror directory |
| `jirafs pull` | fetch Jira issues into local Markdown files (one issue or a scope) |
| `jirafs diff` | show what push would change in Jira (read-only) |
| `jirafs resolve` | resolve a conflict that pull recorded in an issue file |
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

## Creating Issues With An AI

`jirafs` has no LLM dependency. `new --prompt` prints a template plus
instructions, and `new --from` validates and saves the filled document as a
local draft:

```sh
jirafs new story --prompt --idea "users want dark mode" | llm | jirafs new --from -
```

`--from` accepts a file path or `-` for stdin, validates the document (issue
type and summary are required), applies `--project`/`--assignee`, and prints
`jirafs: created <path>`. Invalid documents are rejected with the offending
field and a retry command.

Drafts are created in Jira with `jirafs push`. It is a dry run by default;
`--apply` creates the issue, renames the file to `KEY.md`, marks it synced and
prints the browse URL (`jirafs push` without a file pushes every draft):

```sh
jirafs push jira/add-dark-mode.md            # show what would be created
jirafs push jira/add-dark-mode.md --apply    # POST /rest/api/2/issue
```

Pushing a file that is already synced updates the existing issue instead:
`jirafs push KEY` (or `jirafs push path/KEY.md`).

### Custom fields on create

Some Jira instances require custom fields when creating an issue. Add them to
the draft's frontmatter as `custom_fields:`. Keys are `customfield_NNNNN`;
values may be a string, number, list, or a map such as `{value: "X"}` or
`{id: "123"}`, and are sent to Jira as JSON unchanged:

```yaml
custom_fields:
  customfield_12601:
    value: Platform Team
  customfield_10002: 5
  customfield_10003:
    - id: "10100"
```

To avoid repeating them in every draft, set per-project defaults in
`~/.jirafs/settings.toml`. A draft's own `custom_fields:` win over
`create_defaults` for the same key; templates may include `custom_fields:` too.

```toml
[projects.platform.create_defaults]
customfield_12601 = { value = "Platform Team" }
customfield_10002 = 5
```

Custom fields are sent on create only; they are not synced back.

`jirafs push` (dry run) and `jirafs push --apply` ask Jira's createmeta
endpoint which fields are required and list the ones the payload lacks, with
allowed values and a copy-pasteable `custom_fields:` / `create_defaults`
snippet. `--apply` refuses to create until they are set. If createmeta cannot
be reached, a warning is printed and the push continues.

### Retrying a create is safe

Before creating a draft, `push --apply` writes an intent record
`<mirror_dir>/.jirafs/intents/<draft-name>.yml` (random token, project,
summary, timestamp) and creates the issue with the label `jirafs-<token>`. If
the response is lost (timeout, killed process) the draft stays a draft; the
next `push --apply` searches Jira for `labels = "jirafs-<token>"` first and,
when found, adopts that key instead of creating a duplicate:

```
found PROJ-7 created by an earlier attempt; not creating a duplicate
```

The intent record is deleted once the local `KEY.md` is written, and the label
is removed from the issue (best effort). `jirafs doctor` reports intent records
older than 7 days. Any future write that adds data (for example comments) must
follow the same rule: record the intent, then verify before repeating.

## Initial Scope

- Export Jira issues, sprints, and related metadata into a local filesystem.
- Edit current and future issues locally, then sync changes back to Jira.
- Create new issues locally from typed templates, then upload them.
- Keep historical issues and sprint snapshots as a structured archive.
- Preserve enough metadata to support later features such as kanban views,
  release planning, and archive-driven process analysis.

## Design Principles

- Local files are the main human editing surface.
- Structured frontmatter is the canonical machine-readable layer.
- Markdown sections are used for long-form text fields.
- Sync must be explicit, inspectable, and conflict-aware.
- References to Jira entities must be typed and resolvable.
- The core implementation should work from the command line without requiring
  a Codex skill or another agent layer.

## Documents

- [Architecture](docs/architecture.md)
- [Issue Format](docs/issue-format.md)
- [Sync Model](docs/sync-model.md)
- [References](docs/references.md)
- [Templates](docs/templates.md)
- [CLI](docs/cli.md)
- [Agent Integration](docs/agent-skill-integration.md)
- [Implementation Roadmap](docs/implementation-roadmap.md)
- [Parallel Workstreams](docs/parallel-workstreams.md)
- [Orchestration Model](docs/orchestration-model.md)
- [Verification Policy](docs/verification-policy.md)
- [Development Rules](docs/development-rules.md)
- [Code Style](docs/code-style.md)
- [Validator Contract](docs/validator-contract.md)
- [Mirror Model](docs/mirror-model.md)
- [Settings And Context](docs/settings-and-context.md)
- [Credential Sources](docs/credential-sources.md)
- [Project Selection CLI](docs/project-selection-cli.md)
- [Implementation Packets](docs/implementation-packets.md)
- [Ralph Task Archive](docs/ralph-task-archive.md)
- [Pre-Live Parallel Plan](docs/pre-live-parallel-plan.md)

Read these after the README:

- [CLI](docs/cli.md) for command-level intent
- [Settings And Context](docs/settings-and-context.md) for config shape
- [Credential Sources](docs/credential-sources.md) for auth references
- [Mirror Model](docs/mirror-model.md) for live/archive behavior
- [Sync Model](docs/sync-model.md) for plan/sync behavior

## Implementation Direction

The preferred implementation order is:

1. Define the schema and Markdown codec.
2. Implement registry loading and reference resolution.
3. Implement Jira read/export.
4. Implement local diff/plan.
5. Implement safe sync for a small editable field set.
6. Add templates, bulk export, archive snapshots, and board projections.
7. Add global settings, project selection, and mirror scope management as
   first-class behavior.

The execution plan for that work now lives in:

- [Implementation Roadmap](docs/implementation-roadmap.md)
- [Parallel Workstreams](docs/parallel-workstreams.md)
- [Orchestration Model](docs/orchestration-model.md)
- [Verification Policy](docs/verification-policy.md)
- [Development Rules](docs/development-rules.md)
- [Code Style](docs/code-style.md)
- [Validator Contract](docs/validator-contract.md)
- [Mirror Model](docs/mirror-model.md)
- [Settings And Context](docs/settings-and-context.md)
- [Credential Sources](docs/credential-sources.md)
- [Project Selection CLI](docs/project-selection-cli.md)
- [Implementation Packets](docs/implementation-packets.md)
- [Ralph Task Archive](docs/ralph-task-archive.md)
- [Pre-Live Parallel Plan](docs/pre-live-parallel-plan.md)

## Language Direction

`jirafs` is now planned as a Go implementation.

Why Go:

- single static binary distribution
- minimal runtime assumptions on the operator machine
- lower packaging and interpreter drift risk
- strong long-term fit for a durable CLI utility

The architecture and workflow docs remain valid; the implementation language
changes, not the product model.

## Dependency Policy

`jirafs` should stay conservative about dependencies.

Default rules:

- prefer the Go standard library when it is sufficient
- add a third-party dependency only when it removes meaningful complexity or
  avoids building risky parsing or CLI behavior ourselves
- prefer small, stable, well-maintained libraries over large frameworks
- prefer libraries with narrow scope and low transitive dependency counts
- avoid dependencies that force a large application framework shape onto the
  codebase

Initial likely third-party categories:

- TOML parsing
- YAML parsing when needed for local mirror config files
- Markdown frontmatter handling only if the standard library plus a small
  parser is not enough
- CLI command parsing if the standard library proves too bare for predictable UX

What we want to avoid:

- heavy framework-style CLI stacks
- large indirect dependency trees for simple parsing problems
- generator-driven codebases where handwritten code would stay clearer
- dependencies that make static builds or cross-platform release packaging
  harder

The repository should keep the same verification standard regardless of
language choice: high test coverage, strict local linting, and green CI before
merge.

### Epic link field

Set `epic_link_field = "customfield_10014"` under `[projects.<name>]` in
`~/.jirafs/settings.toml` to export epic links. `JIRAFS_EPIC_LINK_FIELD`
overrides it; `jirafs doctor` shows the effective value.

## Git-backed mirror (opt-in)

`jirafs init <name> ... --git` makes the mirror a git repository so every
change is reviewable and reversible. It sets `git: true` in the mirror's
`mirror.yml` (default `false`; nothing changes when off), runs `git init` in
the mirror directory unless it already sits inside a repository (an existing
repo, including a parent repo, is reused and init says which), and appends
`.jirafs/backup/`, `.jirafs/intents/`, `.jirafs/tmp/` and `.jirafs/base/` to
`.gitignore` (an existing `.gitignore` is appended to, never overwritten).
Re-running is idempotent. Without git on PATH, init fails with an install hint;
a commit without `user.name`/`user.email` hints at `git config --global`.

### Auto-commit after `pull`

With `git: true`, every real `jirafs pull` (not `--dry-run`) commits the files
it wrote with the subject `sync from <host> at <RFC3339 UTC>` and a body
listing the scope (or issue key) and `changed: KEY, KEY` (plus `archived:` and
`moved:` lines when the pull archived or moved issues). Only those exact
paths are staged, never `git add -A`, so unrelated dirty files stay untouched
and unstaged. A pull that changes nothing makes no commit.

jirafs never commits your work implicitly: a file that already has uncommitted
changes is still written by the pull, but is left out of the sync commit and
reported as `left uncommitted: KEY (local edits)`. Review and commit it
yourself. Paths outside the mirror directory are never committed.

The pull itself always succeeds. The commit is skipped with a message on a
detached HEAD (`git switch main`), while a rebase or merge is in progress, or
when the index is locked. If the commit fails, jirafs prints
`pull ok, git commit failed: ...` followed by `git -C <dir> status`.

`mirror.yml` (cursors and `remote_updated`) is committed together with the
issue files whenever jirafs changed it, unless you had uncommitted edits to it
beforehand; then it stays yours to commit.

### Auto-commit after `push --apply`

With `git: true`, a `jirafs push --apply` that wrote at least one issue to Jira
commits the result with the subject `push to <host> at <RFC3339 UTC>`. The body
has one line per issue, such as `KEY: fields summary, labels; transition To Do
-> Done` or `NEW-1: created NEW-1 (from draft.md)`. The commit happens only
after Jira accepted the write and the local file was written; a created draft
stages the new `KEY.md` and the removal of the draft in the same commit.

When some issues fail, only the succeeded ones are committed, the body ends
with `not pushed: KEY (reason)` lines, and jirafs prints
`committed N of M; failed: KEY, KEY`. Pushed files are committed in full (your
edits were just pushed); files you did not push stay untouched. Dry runs, no
remote success, a detached HEAD, a rebase or merge in progress, or a locked
index make no commit. A failing commit never fails the push: jirafs prints
`push ok, git commit failed: ...` followed by `git -C <dir> status`.

### Git mode: merge base and `status`

With `git: true`, the merge base of an issue file (used by `pull`'s three-way
merge and by `push`'s conflict check) is the file as of the newest commit whose
subject starts with `sync from ` or `push to ` (`gitx.LastSyncCommit`), because
that commit is the last known remote state. The same code path falls back to
the stored `.jirafs/base/KEY.md`, and to "no base", when the base cannot be read
from git:

- Commits you made after the sync/push commit never move the base; they count as
  local edits.
- Rewritten history, a missing commit, a file added after the sync commit, or
  unparseable content: the stored base is used.
- Shallow clones and a detached HEAD are fine; only reading is done.

`jirafs status` (offline) adds a `Git:` section with `N local commits not
pushed` (commits after the last sync/push commit touching the mirror) and `N
files with uncommitted edits`, plus `git -C <dir> log --oneline <sha>..HEAD` and
`jirafs push`. With `--porcelain`, unpushed commits appear as
`unpushed-commit<TAB>SHA<TAB>SUBJECT` rows after the `STATE<TAB>KEY<TAB>PATH` rows.
