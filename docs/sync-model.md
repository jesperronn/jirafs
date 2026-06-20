# Sync Model

## Goal

Sync must be explicit, inspectable, and conflict-aware.

The system should never silently overwrite Jira based on a local edit that was
made against stale state.

## Commands

The sync flow should be split into separate stages:

1. export
2. plan
3. sync

### Export

Reads from Jira and writes:

- issue files
- registry files
- snapshot metadata

### Plan

Compares:

- current local issue file
- current remote Jira state
- last-synced state

Outputs:

- creates
- field updates
- comment appends
- transitions
- link changes
- conflicts

### Sync

Applies the approved plan to Jira and updates local metadata.

## Sync Metadata

Each issue file should contain enough metadata to detect conflicts:

- remote issue id
- remote issue key
- remote version or equivalent revision marker
- exported timestamp
- last synced timestamp
- hash of normalized synced content

## Conflict Rules

A conflict exists when:

- the remote issue changed since local export in a field the user edited
- a referenced entity can no longer be resolved
- a requested status transition is not valid
- the issue key or linked remote identity no longer matches the local file

Conflict handling should produce a structured plan, not an immediate failure
deep inside an API call.

## Write Operations

Supported write operations should be explicit:

- create issue
- update issue fields
- append comment
- add link
- remove link
- assign epic
- set parent
- set sprint
- update fix versions
- run workflow transition

Each operation should be visible in the plan output.

## Safe Initial Field Set

The first syncable field set should be intentionally small:

- summary
- description
- labels
- assignee
- parent
- epic
- sprint
- fix versions
- selected custom fields
- comments to add

Status transitions should be supported separately from direct field updates.

## Merge Philosophy

The project should avoid trying to implement a general-purpose text merge for
every field in the first version.

Preferred behavior:

- detect conflicts early
- show the conflicting fields clearly
- let the user re-export, merge manually, or force with intent

## Archive Rules

Historical archive files should usually be treated as non-syncable snapshots.

That can be enforced by:

- directory placement
- `sync.mode: archived`
- a CLI rule that `sync` ignores archive paths by default

## Registry Refresh

Sync planning may depend on up-to-date registries.

Before planning or syncing, the CLI should be able to refresh:

- users
- sprints
- fix versions
- issue type metadata
- workflow transition metadata

## Dry Run

`plan` should be the primary dry-run interface.

`sync --dry-run` may exist, but it should simply reuse the planner and avoid
mutating Jira.
