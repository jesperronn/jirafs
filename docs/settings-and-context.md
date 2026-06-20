# Settings And Context

## Goal

`jirafs` needs one global settings file that lets the CLI resolve Jira context
without forcing every local mirror to live inside a code repository.

The proposed center of that model is:

```text
~/.jirafs/settings.toml
```

This file should answer four questions:

1. Which Jira instances are known?
2. Which Jira projects are known, and which instance owns each one?
3. Which local folders map to those projects?
4. What context should `jirafs` use right now when the user does not pass
   every flag explicitly?

This file is the routing layer for configuration. It is not intended to be the
main secret store.

## Scope

This is global operator state, not per-mirror content.

It should own:

- Jira instance definitions
- auth and credential references
- project definitions
- local mirror locations
- optional local project folders used for auto-detection
- remembered current project
- optional remembered current user identity

It should not own:

- issue content
- registry snapshots
- per-issue sync metadata
- secrets stored in plain text by default

Secret-bearing values should normally live in separate credential sources and be
referenced from settings rather than embedded directly.

## Model

Use three main layers:

1. `instances`
2. `projects`
3. `state`

### Instances

An instance is one Jira base URL plus the auth strategy needed to talk to it.

Each instance should define:

- stable local name such as `work` or `client-a`
- Jira base URL
- auth type
- credential reference, not raw secret material
- optional API base override when needed

### Projects

A project is the main user-facing working context.

Each project should define:

- stable local name such as `platform` or `growth`
- Jira project key such as `PLAT`
- owning instance name
- local mirror directory for Jira data
- zero or more local folder roots that imply this project when `jirafs` runs
  inside them
- optional board ids or other project defaults later

The project entry is where local filesystem usage becomes concrete. This keeps
mirror locations independent from source repositories and allows multiple local
folders to map to the same Jira project.

### State

State is remembered operator context.

It should define:

- `current_project`: the last project explicitly selected by the user
- `current_user`: optional saved local identity for commands that need a Jira
  user reference

`current_user` should be optional because many environments can resolve the
effective Jira user from auth alone.

## Proposed TOML

```toml
version = 1

[state]
current_project = "platform"
current_user = "jesper"

[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"
credential_refs = [
  "file://~/.jirafs/credentials/work-user.toml",
  "env://JIRAFS_WORK_API_TOKEN",
]

[instances.client_a]
base_url = "https://client-a.atlassian.net"
auth_type = "atlassian_api_token"
credential_refs = [
  "op://Engineering/client-a/jira-api-token",
]

[projects.platform]
key = "PLAT"
instance = "work"
mirror_dir = "~/jira/platform"
default_user = "jesper"
local_dirs = [
  "~/src/platform-app",
  "~/src/platform-infra",
  "~/worktrees/platform-docs",
]

[projects.growth]
key = "GROW"
instance = "work"
mirror_dir = "~/jira/growth"
local_dirs = [
  "~/src/growth-site",
]

[projects.client_portal]
key = "PORTAL"
instance = "client_a"
mirror_dir = "~/jira/client-portal"
local_dirs = [
  "~/clients/portal-app",
]
```

## Field Rules

- `version` is required and guards future migration logic.
- `instances.<name>.base_url` is required and must be absolute.
- `instances.<name>.auth_type` is required.
- `instances.<name>.credential_refs` is preferred and may contain one or more
  references resolved in order.
- `instances.<name>.credential_ref` may be retained as a compatibility alias if
  the implementation wants a single-ref shorthand.
- `projects.<name>.key` is required and should be the canonical Jira project
  key.
- `projects.<name>.instance` must reference a declared instance.
- `projects.<name>.mirror_dir` is required and should point to the root folder
  where that project's Jira mirror lives.
- `projects.<name>.local_dirs` is optional but recommended for auto-detection.
- `state.current_project` is optional but, if present, must reference a
  declared project.
- `state.current_user` is optional.

## Credential References

The settings file should prefer indirection for secrets.

Supported patterns can be implementation-defined, but the model should assume
references such as:

- `env://JIRAFS_WORK_TOKEN`
- `file://~/.jirafs/credentials/work.toml`
- `op://Engineering/jirafs-work/api-token`
- `command://jirafs-credential-work`

The CLI resolves the reference at runtime. The settings file should not require
copying API tokens into TOML.

For the first implementation, one instance should normally declare:

- one credential source, or
- a short ordered list of credential sources merged left-to-right

That supports split configuration such as:

- non-secret identity in one local file
- token from an environment variable or secret manager

Example:

```toml
[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"
credential_refs = [
  "file://~/.jirafs/credentials/work-user.toml",
  "env://JIRAFS_WORK_API_TOKEN",
]
```

See [Credential Sources](credential-sources.md) for the provider model and
merge rules.

## Context Resolution

`jirafs` should resolve runtime context in a strict precedence order.

The resolved context should include:

- project name
- Jira project key
- instance name
- Jira base URL
- auth configuration
- mirror directory
- effective user identity when available

### Resolution Order

1. Explicit CLI flags win.
2. If no project was passed, try auto-detection from the current working
   directory.
3. If no folder match exists, use `state.current_project` when set.
4. If exactly one project is configured, use it as the implicit fallback.
5. Otherwise fail with a clear error asking for `--project` or configuration.

### Explicit Flags

The CLI should allow explicit overrides such as:

- `--project`
- `--instance` for advanced or debugging flows
- `--user`
- `--mirror-dir` only when a command truly needs it

`--project` should normally be enough because it selects the instance and mirror
implicitly through project config.

### Auto-Detection From Current Directory

Auto-detection should match the current working directory against every
configured `local_dirs` entry after path expansion and normalization.

Rules:

- A project matches when the current directory is equal to or nested under one
  of its configured `local_dirs`.
- Choose the most specific match by longest normalized path prefix.
- If two projects tie on the same matched prefix length, fail as ambiguous.
- Do not depend on Git metadata for detection. A plain folder match is enough.

This keeps detection useful for code repos, docs repos, and non-repo folders.

### Effective User Resolution

Resolve the effective Jira user in this order:

1. `--user`
2. `projects.<name>.default_user` when present
3. `state.current_user`
4. auth-derived remote identity if supported by the client
5. unset

Commands that require a concrete user reference should fail clearly if no user
can be resolved.

## Remembered State Rules

The CLI should update `state.current_project` when the user explicitly selects a
project, for example through `--project` or a future `jirafs use <project>`
command.

It should not silently rewrite remembered state just because folder
auto-detection happened during one command. Auto-detection is a runtime hint;
remembered state is an explicit operator choice.

`state.current_user` should only be changed through an explicit command or
settings edit.

## Mirror Directory Rules

`mirror_dir` is a Jira workspace root, not a source repository root.

That directory may contain:

- exported issues
- drafts
- registries
- archive data
- project-local templates

It should be valid for:

- two different code repos to map to one Jira project
- one Jira project to have no code repo at all
- operators to run `jirafs` directly inside the mirror directory

## Validation And Failure Modes

Settings loading should fail early on:

- unknown project references
- unknown instance references
- duplicate normalized `local_dirs` across projects when they would create
  unavoidable ambiguity
- invalid or relative `base_url`
- empty `mirror_dir`
- invalid or unsupported credential reference schemes

Commands should fail with context-specific errors when:

- no project can be resolved
- multiple projects match the current directory equally
- a credential reference cannot be resolved
- the resolved project has no usable instance config

## Future Extensions

This shape leaves room for later additions without changing the core model:

- per-project board ids
- project-specific custom field mappings
- per-instance API version settings
- multiple credential refs for different auth flows
- named profiles for separate operator environments

## Summary

The main design choice is to make `project` the default unit of context while
keeping `instance` and credentials behind it. That gives `jirafs` one clear
answer for most commands, supports multiple Jira sites, and allows folder-based
auto-detection even when the user is working outside the Jira mirror itself.
