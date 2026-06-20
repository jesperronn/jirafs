# Parallel Workstreams

## Context

`jirafs` is currently document-heavy. This split defines the intended parallel
implementation plan for the future codebase while keeping ownership boundaries
strict enough that multiple workers can move without colliding.

Assumed target layout:

```text
src/jirafs/
  schema/
  codec/
  config/
  context/
  mirror/
  registry/
  references/
  jira/
  export/
  plan/
  sync/
  cli/
  templates/
  board/
  archive/
  agents/
tests/
```

## Shared Rules

- One workstream owns one path prefix. No drive-by edits outside that prefix.
- Shared scaffolding files have a single integration owner.
- Existing docs are treated as contested territory. Do not edit `docs/*.md`
  outside your assigned coordination/doc task unless explicitly agreed.
- All streams are bound by [Verification Policy](verification-policy.md).
- Cross-stream contract changes require a short written notice in the issue or
  PR before code lands.
- Public interfaces must be imported across streams; sibling internals are not
  fair game.
- Incomplete work should land behind hidden commands, stubs, or feature flags
  rather than blocking parallel streams for long-lived branches.

## Workstreams

| ID | Workstream | Suggested owner | Exclusive write scope | Depends on | Can run concurrently with |
| --- | --- | --- | --- | --- | --- |
| WS0 | Foundation and integration | Agent A or tech lead | `pyproject.toml`, `src/jirafs/__init__.py`, `src/jirafs/interfaces/**`, `tests/conftest.py`, `tests/contracts/**`, CI/bootstrap files | none | all other streams once contracts are stubbed |
| WS1 | Schema and markdown codec | Agent B | `src/jirafs/schema/**`, `src/jirafs/codec/**`, `tests/schema/**`, `tests/codec/**` | WS0 interface stubs | WS2, WS3, WS5 |
| WS2 | Settings, context, registry, and reference resolution | Agent C | `src/jirafs/config/**`, `src/jirafs/context/**`, `src/jirafs/registry/**`, `src/jirafs/references/**`, matching tests | WS0 interface stubs | WS1, WS3, WS6 |
| WS3 | Jira client and export read path | Agent D | `src/jirafs/jira/**`, `src/jirafs/export/**`, matching tests | WS0 interface stubs; WS1 types for issue shape | WS1, WS2, WS5 |
| WS4 | Planner and sync applier | Agent E | `src/jirafs/plan/**`, `src/jirafs/sync/**`, matching tests | WS1, WS2, WS3 | WS5 after contracts are frozen |
| WS5 | Mirror manager and archive sweep | Agent F | `src/jirafs/mirror/**`, `src/jirafs/archive/**`, matching tests | WS2, WS3, WS4 | WS6 after contracts are frozen |
| WS6 | CLI surface and user-visible command flow | Agent G | `src/jirafs/cli/**`, `tests/cli/**` | WS0 for command skeletons; then WS1-WS5 adapters | WS1, WS2, WS3; limited overlap with WS4 |
| WS7 | Templates, board views, and agent helpers | Agent H or later phase | `src/jirafs/templates/**`, `src/jirafs/board/**`, `src/jirafs/agents/**`, matching tests | WS1, WS2, WS6; parts of WS4 and WS5 for sync-aware features | starts after core contracts stabilize |

## Ownership Boundaries

### WS0 Foundation and Integration

- Owns package root, dependency declarations, test harness, contract-test
  directory, and any shared abstract interfaces.
- Is the only stream allowed to edit cross-cutting bootstrap files.
- Must keep its changes minimal; it should create seams, not implement feature
  logic that belongs to other streams.

### WS1 Schema and Markdown Codec

- Owns canonical in-memory models and deterministic Markdown round-tripping.
- Must publish stable constructors and serializers early because every other
  stream depends on these shapes.
- Must not reach into Jira HTTP details or CLI parsing.

### WS2 Settings, Context, Registry, and References

- Owns typed refs for users, sprints, fix versions, projects, epics, and
  configuration loading/mapping.
- Owns `~/.jirafs/settings.toml`, current-project memory, cwd-based project
  detection, and effective user resolution.
- Must expose resolution APIs that WS3 and WS4 can consume without editing WS2
  internals.

### WS3 Jira Client and Export

- Owns remote reads, search/export flows, and Jira transport concerns.
- Must not embed planning or sync-decision logic; it returns normalized remote
  data for WS4.

### WS4 Planner and Sync Applier

- Owns diffing, conflict detection, plan generation, and safe write execution.
- Must treat WS1 models, WS2 references, and WS3 remote snapshots as inputs.
- Must not change schema or resolver behavior inline; contract gaps go back to
  the owning stream.

### WS5 Mirror Manager and Archive Sweep

- Owns named mirror scopes, mirror membership, refresh orchestration, and local
  archive movement rules.
- Must not own Jira transport or document schema details.

### WS6 CLI

- Owns command layout, argument parsing, output shaping, prompting behavior,
  and orchestration.
- May add adapter code only inside `src/jirafs/cli/**`.
- Must not pull business logic down into commands just to unblock itself.

### WS7 Higher-Level Features

- Owns non-MVP layers that depend on the core stack being stable.
- Should stay out of core sync paths until WS4 has landed.

## Dependencies and Merge Order

1. Merge WS0 first, but keep it thin: package skeleton, interface seams, test
   harness, and contract-test locations.
2. Merge WS1 next or in parallel with WS2, as soon as the canonical issue model
   and codec API are testable.
3. Merge WS2 once resolver/config contracts are stable enough for planning and
   export consumers.
4. Merge WS3 after it targets WS1 models instead of inventing its own shapes.
5. Merge WS5 after WS2 and WS3 have stable enough contracts for mirror refresh
   and archive membership rules.
6. Merge WS6 incrementally: command shells can land early, but full wiring
   should wait for the called services to exist behind stable interfaces.
7. Merge WS4 after WS1, WS2, and WS3 have frozen their first public contracts.
   This is the highest-risk integration point and should not race contract churn.
8. Merge WS7 last, after core read-plan-sync flows are green.

## What Can Run Concurrently

- WS1 and WS2 can run in parallel immediately after WS0 publishes stubs.
- WS3 can start in parallel with WS1 and WS2 if it consumes provisional schema
  interfaces instead of inventing parallel models.
- WS5 can start shallow mirror membership and archive policy fixtures once WS2
  fixes settings and context contracts.
- WS6 can build command shells, JSON output framing, and dependency injection in
  parallel with WS1-WS3 by mocking service interfaces owned by WS0.
- WS4 can start design and fixture work early, but real implementation should
  begin only once WS1-WS3 public contracts stop moving daily.
- WS7 can start isolated template work once WS1 and WS2 are stable, but board
  and agent surfaces should wait for WS6 command patterns.

## Coordination Rules

- Use a single contract note per stream for exported types, exceptions, and
  service entrypoints. Update it before widening or breaking an interface.
- When a change touches a shared contract, the owning stream lands the contract
  first; dependent streams follow with adapter changes.
- Do not edit another stream's tests to force compatibility. Open an interface
  request instead.
- Rebase or merge from main after every contract landing, not only at the end
  of the feature.
- Keep PRs path-local. If a PR needs files from two ownership zones, split it
  or route it through WS0 as an intentional integration change.
- Preserve docs isolation. During active parallel development, queue edits to
  `docs/architecture.md`, `docs/cli.md`, and related files behind a single docs
  owner instead of letting every stream update them opportunistically.

## Recommended Team Shape

- 4 workers: combine WS0 with WS6 under one integration owner, and defer WS7.
- 5 workers: keep WS0 separate; combine WS5 and WS6 until core sync is stable.
- 6 or more workers: staff WS0 through WS6 independently; treat WS7 as a later
  tranche unless the MVP already has green contract tests.

## Exit Criteria Per Phase

- Phase 1 complete when WS0-WS3 have stable contract tests and `export` can
  round-trip one issue into the local model without CLI hacks.
- Phase 2 complete when WS4, WS5, and WS6 deliver mirror refresh, `plan`, and
  `sync` for the initial safe field set with explicit conflict output.
- Phase 3 complete when WS7 adds templates, archive flows, and board/agent
  features without reopening core ownership boundaries.
