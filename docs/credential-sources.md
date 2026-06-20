# Credential Sources

## Goal

`jirafs` should separate shared operator configuration from secret material.

The main settings file:

```text
~/.jirafs/settings.toml
```

should normally store references to credential sources rather than raw secrets.

This keeps:

- project and instance definitions easy to review and share
- secret material out of the main config
- credentials flexible across local files, environment variables, and secret
  managers

## Layout

A typical user-level layout may look like:

```text
~/.jirafs/
  settings.toml
  credentials/
    work-user.toml
    client-a.toml
  state/
    current.json
```

The credential files under `~/.jirafs/credentials/` are local operator files
and should usually stay out of Git.

## Provider Model

The first implementation should support a small fixed provider set.

Recommended schemes:

- `env://VAR_NAME`
- `file://~/.jirafs/credentials/work.toml`
- `op://vault/item/field`
- `command://jirafs-credential-work`

This is preferable to arbitrary plugin hooks because it is:

- easier to test
- easier to document
- easier to reason about securely
- more portable across machines

## Provider Semantics

### `env://`

Reads one value from an environment variable.

Good for:

- CI
- local shell-managed tokens
- secret injection from wrapper scripts

Example:

```toml
credential_refs = ["env://JIRAFS_WORK_API_TOKEN"]
```

### `file://`

Reads a local file from disk, usually a TOML file with one or more auth fields.

Good for:

- split local config
- non-secret identity data plus secret token layering
- operator-managed local credentials outside Git

Example:

```toml
credential_refs = ["file://~/.jirafs/credentials/work-user.toml"]
```

Possible file content:

```toml
user_email = "jesper@example.com"
```

or:

```toml
api_token = "..."
```

### `op://`

Reads a secret from 1Password or a similar secret manager path model.

Good for:

- managed team secrets
- avoiding token files on disk

Example:

```toml
credential_refs = ["op://Engineering/jira-work/api-token"]
```

The exact runtime integration can be defined later, but the config model should
reserve the scheme now.

### `command://`

Runs a local command and reads its stdout as a credential payload or value.

Good for:

- shell-mediated secret retrieval
- custom local integrations
- replacement for vague "call a function" ideas

Example:

```toml
credential_refs = ["command://jirafs-credential-work"]
```

This is the preferred general escape hatch because it is explicit and testable.

## Merge Model

Instances should support multiple credential references.

Recommended field:

```toml
credential_refs = [
  "file://~/.jirafs/credentials/work-user.toml",
  "env://JIRAFS_WORK_API_TOKEN",
]
```

Merge rule:

- resolve references in order
- later sources override earlier sources

That allows useful split setups such as:

- base identity from file
- secret token from env or 1Password

## Supported Payload Shapes

Resolved credential sources should normalize into a small field set such as:

- `user_email`
- `api_token`
- `bearer_token`
- optional `account_id`

This keeps the auth model tight and avoids provider-specific credential shapes
leaking into the rest of the system.

## Auth Examples

### Split File Plus Env

```toml
[instances.work]
base_url = "https://jira.example.com"
auth_type = "atlassian_api_token"
credential_refs = [
  "file://~/.jirafs/credentials/work-user.toml",
  "env://JIRAFS_WORK_API_TOKEN",
]
```

### File Only

```toml
[instances.client_a]
base_url = "https://client-a.atlassian.net"
auth_type = "bearer_token"
credential_refs = [
  "file://~/.jirafs/credentials/client-a.toml",
]
```

### 1Password Only

```toml
[instances.work]
base_url = "https://jira.example.com"
auth_type = "bearer_token"
credential_refs = [
  "op://Engineering/jira-work/api-token",
]
```

## Git And Sharing Rules

- `settings.toml` may be shareable if it contains only references and
  non-secret project metadata.
- files under `~/.jirafs/credentials/` should usually not be committed.
- teams can commit safe example settings and keep the secret-bearing files
  local.

## Failure Modes

Credential resolution should fail early and clearly when:

- a provider scheme is unknown
- an environment variable is missing
- a referenced file does not exist
- a secret-manager lookup fails
- a command provider exits non-zero
- the merged credential payload is incomplete for the selected auth type

## Default Standard

- keep `settings.toml` as a routing file, not a secret store
- prefer a fixed provider set over arbitrary plugin handlers
- support `env://`, `file://`, `op://`, and `command://`
- allow multiple ordered credential refs with later values overriding earlier
  ones
- keep secret-bearing local files outside Git by default
