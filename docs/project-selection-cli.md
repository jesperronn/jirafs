# Project Selection CLI

## Goal

`jirafs` must make project selection predictable in both human and scripted
flows.

The CLI needs to answer four questions consistently:

- which Jira project a command will operate on
- when the CLI should reuse a previously selected project
- what should happen inside a locally configured project workspace
- when commands should require `--project` versus prompting the user

## Terminology

- Jira project: a Jira project key such as `ABC` or `PLAT`
- local project folder: a local workspace whose `jira/config.yaml` declares a
  default Jira project
- current project memory: the last project explicitly or implicitly selected by
  the user, stored in user state outside the repo

Project selection is about Jira projects, not local directory names.

## Resolution Order

When a command needs a project, `jirafs` resolves it in this order:

1. explicit `--project <KEY>`
2. project inferred from an explicit issue key such as `ABC-123`
3. project pinned by the nearest local `jira/config.yaml`
4. remembered current project from user state
5. interactive prompt, only when stdin is a TTY
6. hard failure

The first successful match wins.

## Local Workspace Rules

`jirafs` should walk upward from the current working directory looking for the
nearest `jira/config.yaml`.

If that config declares a default Jira project, that project becomes the active
project for the current command unless a higher-precedence source overrides it.

Recommended config shape:

```yaml
project:
  default: ABC
```

Behavior inside a configured local project folder:

- do not prompt just because remembered global state points to another project
- treat the folder-pinned project as the default context for project-scoped
  commands
- still allow `--project XYZ` to override the folder for a single command
- if the command writes local files, prefer refusing cross-project writes unless
  the target path is explicitly outside the current workspace

That last rule prevents a user from standing in an `ABC` workspace and
accidentally creating `XYZ` drafts into the wrong local tree.

## Current Project Memory

`jirafs` should keep lightweight user-level memory of the current project for
interactive convenience outside pinned workspaces.

Suggested behavior:

- store only the project key and last-used timestamp
- update memory whenever project resolution succeeds from `--project`, issue-key
  inference, local config, or interactive selection
- treat memory as a fallback, not as stronger than local workspace config
- keep memory outside the repo so it does not create team-local churn

Suggested state shape:

```json
{
  "current_project": "ABC",
  "updated_at": "2026-06-20T12:34:56Z"
}
```

Memory should not be used when:

- `--no-project-memory` is passed
- the command is explicitly cross-project
- the command is running in a mode that promises no hidden defaults

## Interactive Behavior

Interactive mode means stdin and stderr are attached to a TTY.

If a project is still unresolved after checking explicit overrides, issue-key
inference, local config, and memory, `jirafs` should prompt.

The prompt should:

- show the remembered project first when one exists
- show recently used or configured projects before the full registry list
- accept a project key directly
- allow cancel with a non-zero exit

Example prompt:

```text
$ jirafs new story
No project was specified.
Select a Jira project:
  1. ABC  Platform Core   (current)
  2. PLAT Developer Experience
  3. OPS  Operations
>
```

After a successful choice, `jirafs` should use that project for the current
command and update current project memory.

## Non-Interactive Behavior

Non-interactive mode means no TTY, `--non-interactive`, or machine-readable
output modes that forbid prompts.

In non-interactive mode, `jirafs` must never prompt.

If project resolution cannot complete from:

- `--project`
- issue-key inference
- local workspace config
- current project memory, when allowed

then the command must fail with a clear error and a short fix message.

Example:

```text
$ jirafs new story --non-interactive
error: no Jira project could be resolved
hint: pass --project <KEY> or run inside a configured jirafs workspace
```

## Command Policy

Commands fall into three groups.

### 1. Project-Required Commands

These commands should expose `--project` and must resolve exactly one project:

- `jirafs new`
- `jirafs board`
- `jirafs export backlog`
- `jirafs export sprint current`
- `jirafs registry refresh` when refreshing project-scoped metadata

If interactive and unresolved, they may prompt.
If non-interactive and unresolved, they must fail.

### 2. Project-Derivable Commands

These commands may not need `--project` because the target implies it:

- `jirafs export issue ABC-123`
- `jirafs plan jira/live/issues/ABC-123.md`
- `jirafs sync jira/live/issues/ABC-123.md`

They should still accept `--project` for validation or disambiguation, but
issue-key or file metadata inference should normally win without prompting.

If both an inferred project and `--project` are present and they disagree, the
command must fail instead of silently picking one.

### 3. Cross-Project Or Global Commands

These commands should avoid hidden project defaults unless the user explicitly
asks to scope them:

- `jirafs registry show projects`
- `jirafs export jql 'assignee = currentUser()'`
- `jirafs archive issue ABC-111`

For these commands:

- `--project` should narrow scope when that makes sense
- prompting should be avoided unless the command truly requires one project to
  continue
- global commands should remain global by default

## Override Rules

`--project` is the only explicit CLI override and should be available on any
command where one-project scope is meaningful.

Rules:

- `--project` overrides local config and current project memory
- `--project` does not override an explicit issue key that names a different
  project; that is a user error
- `--project` should be echoed in human-readable output when it materially
  affects scope
- JSON output should include the resolved project key whenever a single-project
  command runs

Example:

```text
$ jirafs board --project PLAT
Project: PLAT
Sprint: current
...
```

## Startup Behavior

When `jirafs` starts inside a configured local project folder:

1. discover the nearest `jira/config.yaml`
2. load `project.default`
3. make that project active for the command
4. update current project memory to the same project after resolution succeeds

When `jirafs` starts outside any configured local project folder:

1. check whether the command target already implies a project
2. otherwise fall back to current project memory
3. otherwise prompt in interactive mode or fail in non-interactive mode

## Example Flows

### Interactive Outside A Workspace

```text
$ pwd
/tmp

$ jirafs new bug
No project was specified.
Select a Jira project:
  1. ABC  Platform Core   (current)
  2. OPS  Operations
> 2

Created draft: jira/live/drafts/OPS-new-bug.md
```

### Interactive Inside A Pinned Workspace

```text
$ pwd
~/work/customer-portal

$ cat jira/config.yaml
project:
  default: ABC

$ jirafs new story
Created draft: jira/live/drafts/ABC-new-story.md
```

No prompt should appear in that flow.

### One-Off Override Inside A Pinned Workspace

```text
$ jirafs board --project OPS
Project: OPS
Sprint: current
...
```

That command should use `OPS` for the current invocation and update current
project memory to `OPS`, but it must not rewrite `jira/config.yaml`.

### Non-Interactive Scripted Use

```text
$ jirafs export backlog --non-interactive --project ABC --json
{
  "project": "ABC",
  "issues": [...]
}
```

### Conflicting Explicit Inputs

```text
$ jirafs export issue ABC-123 --project OPS
error: issue ABC-123 belongs to project ABC but --project was OPS
```

## Default Standard

- prefer explicit `--project` in scripts
- prefer pinned local config inside project workspaces
- use current project memory only as a convenience fallback
- never prompt in non-interactive mode
- fail on conflicting project signals instead of guessing
