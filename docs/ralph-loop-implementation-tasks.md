# Ralph Loop Implementation Tasks

## Goal

This file splits the roadmap packets into ralph-loop-sized implementation
steps. Each task is intended for one builder iteration: one owner, one narrow
behavior, one validation target, and one clear handoff.

Use this together with:

- [Implementation Packets](implementation-packets.md)
- [Implementation Roadmap](implementation-roadmap.md)
- [Orchestration Model](orchestration-model.md)
- [Parallel Workstreams](parallel-workstreams.md)
- [Verification Policy](verification-policy.md)
- [Development Rules](development-rules.md)

## Loop Rules

- Pick only the first unchecked task whose dependencies are complete.
- Complete exactly one task per ralph iteration, then stop.
- Run `bin/test` and `bin/lint` after the final diff.
- Add or update tests in the same task as the code they prove.
- Keep each task inside its owned paths unless the task explicitly names an
  integration file.
- Mark the task complete only after validation passes.
- Record a handoff using the format from [Orchestration Model](orchestration-model.md).
- If using delegate implementors, run at most two at the same time.

## Task Ledger

### Foundation

- [ ] B001: Create Go module and package skeleton.
  Own: `go.mod`, `cmd/jirafs/**`, `internal/**`, `tests/**`, `bin/**`.
  Acceptance: `jirafs` builds, package layout is documented by directory names,
  and existing `bin/test` plus `bin/lint` still pass.

- [ ] B002: Add CLI help shell for documented top-level commands.
  Own: `cmd/jirafs/**`, `internal/cli/**`, `tests/cli/**`.
  Depends on: B001.
  Acceptance: help lists `init`, `export`, `plan`, `sync`, `new`, `registry`,
  `board`, and `archive` without implementing feature behavior.

- [ ] B003: Add fixture and golden-test helpers.
  Own: `tests/**`, `internal/testutil/**`.
  Depends on: B001.
  Acceptance: tests can load fixture files and compare exact output with useful
  diffs.

### Settings And Context

- [ ] B010: Define structured settings errors.
  Own: `internal/config/**`, `tests/config/**`.
  Depends on: B001.
  Acceptance: validation errors expose stable codes and human messages.

- [ ] B011: Parse `~/.jirafs/settings.toml`.
  Own: `internal/config/**`, `tests/config/**`.
  Depends on: B010.
  Acceptance: instances and projects load from TOML with path expansion.

- [ ] B012: Validate settings relationships.
  Own: `internal/config/**`, `tests/config/**`.
  Depends on: B011.
  Acceptance: duplicate keys, missing instances, invalid project references, and
  invalid mirror directories fail with structured errors.

- [ ] B013: Resolve project from explicit `--project`.
  Own: `internal/context/**`, `tests/context/**`.
  Depends on: B011.
  Acceptance: explicit project selection wins over every other source.

- [ ] B014: Resolve project from cwd mappings.
  Own: `internal/context/**`, `tests/context/**`.
  Depends on: B013.
  Acceptance: most-specific folder match wins and ambiguous matches fail
  clearly.

- [ ] B015: Read and write remembered current project.
  Own: `internal/context/**`, `tests/context/**`.
  Depends on: B013.
  Acceptance: remembered state is used only when higher-precedence inputs are
  absent.

- [ ] B016: Add non-interactive unresolved-context failure.
  Own: `internal/context/**`, `tests/context/**`.
  Depends on: B013.
  Acceptance: non-interactive commands fail clearly instead of prompting.

### Schema

- [ ] B020: Define typed reference value objects.
  Own: `internal/schema/**`, `tests/schema/**`.
  Depends on: B001.
  Acceptance: users, projects, statuses, sprints, fix versions, epics, and
  issue keys use one shared typed-ref representation.

- [ ] B021: Define issue document model.
  Own: `internal/schema/**`, `tests/schema/**`.
  Depends on: B020.
  Acceptance: the model covers required frontmatter and fixed Markdown sections
  from [Issue Format](issue-format.md).

- [ ] B022: Define sync metadata model.
  Own: `internal/schema/**`, `tests/schema/**`.
  Depends on: B021.
  Acceptance: remote version, content hash, timestamps, and syncable state are
  represented with validation.

- [ ] B023: Define registry models.
  Own: `internal/schema/**`, `tests/schema/**`.
  Depends on: B020.
  Acceptance: users, projects, statuses, sprints, and fix versions match
  [References](references.md).

- [ ] B024: Define plan and operation models.
  Own: `internal/schema/**`, `tests/schema/**`.
  Depends on: B021.
  Acceptance: editable changes and conflicts are typed without Jira transport
  dependencies.

### Markdown Codec

- [ ] B030: Parse issue frontmatter into schema models.
  Own: `internal/codec/**`, `tests/codec/**`.
  Depends on: B021.
  Acceptance: valid synced and draft frontmatter parse; invalid frontmatter
  returns structured errors.

- [ ] B031: Parse fixed issue sections.
  Own: `internal/codec/**`, `tests/codec/**`.
  Depends on: B030.
  Acceptance: known sections map to canonical fields and unknown sections fail.

- [ ] B032: Render issue documents deterministically.
  Own: `internal/codec/**`, `tests/codec/**`.
  Depends on: B031.
  Acceptance: field and section ordering is stable.

- [ ] B033: Add round-trip golden fixtures.
  Own: `tests/codec/**`.
  Depends on: B032.
  Acceptance: parsing and rendering valid synced and draft fixtures is stable on
  the second render.

### Registry And References

- [ ] B040: Load registry files from a mirror.
  Own: `internal/registry/**`, `tests/registry/**`.
  Depends on: B011 and B023.
  Acceptance: each registry family loads independently with structured file
  errors.

- [ ] B041: Resolve typed references.
  Own: `internal/references/**`, `tests/references/**`.
  Depends on: B040.
  Acceptance: all typed refs resolve to Jira ids through registry data.

- [ ] B042: Add missing and ambiguous reference failures.
  Own: `internal/references/**`, `tests/references/**`.
  Depends on: B041.
  Acceptance: failures include the ref type, lookup value, and candidate context
  where available.

### Jira Read Path

- [ ] B050: Add Jira client interface and fake transport.
  Own: `internal/jira/**`, `tests/jira/**`.
  Depends on: B021.
  Acceptance: tests can supply Jira JSON without network access.

- [ ] B051: Fetch one Jira issue into a remote snapshot.
  Own: `internal/jira/**`, `tests/jira/**`.
  Depends on: B050.
  Acceptance: one issue payload maps to normalized remote data.

- [ ] B052: Normalize one remote issue into the canonical issue model.
  Own: `internal/export/**`, `tests/export/**`.
  Depends on: B021 and B051.
  Acceptance: linked issues stay shallow typed references.

- [ ] B053: Search issues for a named mirror scope.
  Own: `internal/jira/**`, `tests/jira/**`.
  Depends on: B050.
  Acceptance: a fake `my-issues` or `current-sprint` query returns deterministic
  issue keys.

### Planner And Sync

- [ ] B060: Build no-op plan detection.
  Own: `internal/plan/**`, `tests/plan/**`.
  Depends on: B024 and B052.
  Acceptance: unchanged local and remote models produce an empty typed plan.

- [ ] B061: Plan editable field changes.
  Own: `internal/plan/**`, `tests/plan/**`.
  Depends on: B060.
  Acceptance: summary, description, labels, assignee, status, sprint, and fix
  version changes become typed operations.

- [ ] B062: Plan stale-state conflicts.
  Own: `internal/plan/**`, `tests/plan/**`.
  Depends on: B060.
  Acceptance: stale remote version or content hash produces conflicts instead
  of operations.

- [ ] B063: Apply a validated no-op or field-change plan.
  Own: `internal/sync/**`, `tests/sync/**`.
  Depends on: B061.
  Acceptance: sync only accepts validated plan objects.

- [ ] B064: Reject unsafe sync before mutation.
  Own: `internal/sync/**`, `tests/sync/**`.
  Depends on: B063.
  Acceptance: archive paths, unresolved refs, stale state, and invalid
  transitions fail before Jira mutation.

### Mirror, CLI, Archive, And Board

- [ ] B070: Define mirror membership model.
  Own: `internal/mirror/**`, `tests/mirror/**`.
  Depends on: B011 and B014.
  Acceptance: explicit imports and named scopes can coexist with explainable
  membership reasons.

- [ ] B071: Calculate archive eligibility.
  Own: `internal/mirror/**`, `tests/mirror/**`.
  Depends on: B070.
  Acceptance: resolved issues become archive-eligible only after leaving live
  scope; pinned or unsynced issues stay live.

- [ ] B080: Add `jirafs use` command.
  Own: `internal/cli/**`, `tests/cli/**`.
  Depends on: B015 and B016.
  Acceptance: interactive and non-interactive project selection behavior matches
  [Project Selection CLI](project-selection-cli.md).

- [ ] B081: Add `jirafs mirror refresh` command shell.
  Own: `internal/cli/**`, `tests/cli/**`.
  Depends on: B052, B053, and B070.
  Acceptance: command uses project context and calls mirror refresh services
  through interfaces.

- [ ] B082: Add `jirafs mirror archive-sweep` command shell.
  Own: `internal/cli/**`, `tests/cli/**`.
  Depends on: B071.
  Acceptance: command reports eligible archive actions without mutating unless
  explicitly requested.

- [ ] B090: Move archive-eligible issues.
  Own: `internal/archive/**`, `tests/archive/**`.
  Depends on: B064 and B071.
  Acceptance: archive movement preserves snapshots and live membership rules.

- [ ] B091: Build local board grouping.
  Own: `internal/board/**`, `tests/board/**`.
  Depends on: B052 and B070.
  Acceptance: board views group local mirror issues by status, assignee, and
  epic.

## Review Notes

The original implementation packets are suitable for orchestration, but not for
direct builder assignment. The split above keeps each step small enough for a
ralph iteration by separating contract definition, parsing, rendering,
resolution, transport, planning, mutation safety, and CLI wiring.

The first safe two-implementor wave is:

- B001, then B002 or B003
- B020 once B001 exists

Do not start P4/B030 work until B021 has landed, and do not start planner or
sync work until the schema, reference, and export contracts have stable tests.
