# Implementation Packets

## Goal

These packets are the next concrete handoff units for implementation agents.

Each packet is:

- path-local
- bounded enough for one worker
- explicit about dependencies
- paired with acceptance criteria and verification expectations

Read this together with:

- [Implementation Roadmap](implementation-roadmap.md)
- [Parallel Workstreams](parallel-workstreams.md)
- [Verification Policy](verification-policy.md)
- [Development Rules](development-rules.md)

## Packet P0: Settings Skeleton

Own:

- `src/jirafs/config/**`
- `tests/config/**`

Implement:

- settings loader for `~/.jirafs/settings.toml`
- schema validation for instances, projects, and state
- path expansion and normalization helpers

Acceptance criteria:

- invalid settings fail with structured errors
- multiple Jira instances are supported
- `mirror_dir` can live outside any code repo
- unit coverage for all validation branches

## Packet P1: Context Resolver

Own:

- `src/jirafs/context/**`
- `tests/context/**`

Implement:

- project resolution precedence
- cwd-to-project detection
- current-project memory read and write
- effective user resolution policy

Depends on:

- P0

Acceptance criteria:

- `--project` beats remembered state
- cwd detection uses the most specific configured path
- ambiguous folder matches fail clearly
- non-interactive unresolved context fails without prompting

## Packet P2: Mirror Scope Model

Own:

- `src/jirafs/mirror/**`
- `tests/mirror/**`

Implement:

- named mirror scope definitions
- mirror membership model
- archive eligibility rules
- separation of refresh, sync, and archive phases

Depends on:

- P0
- P1

Acceptance criteria:

- explicit issue imports and named scopes can coexist
- membership reasons are explainable
- resolved issues can become archive-eligible only after leaving live scope
- pinned or unsynced issues remain live

## Packet P3: Canonical Issue And Registry Models

Own:

- `src/jirafs/schema/**`
- `tests/schema/**`

Implement:

- issue document model
- sync metadata model
- registry models
- operation and plan models

Acceptance criteria:

- issue and registry contracts match the docs
- field ownership is explicit
- invalid documents fail with structured validation errors

## Packet P4: Markdown Codec

Own:

- `src/jirafs/codec/**`
- `tests/codec/**`

Implement:

- deterministic parse/render for issue docs
- golden-file fixtures

Depends on:

- P3

Acceptance criteria:

- render is stable on second pass
- draft and synced issue shapes both round-trip
- unknown sections fail explicitly

## Packet P5: Registry And Reference Resolution

Own:

- `src/jirafs/registry/**`
- `src/jirafs/references/**`
- matching tests

Implement:

- registry loading
- typed reference resolution
- structured missing and ambiguous ref errors

Depends on:

- P0
- P3

Acceptance criteria:

- users, sprints, fix versions, projects, and statuses resolve through typed refs
- Jira ids remain behind the resolver boundary

## Packet P6: Jira Read Path

Own:

- `src/jirafs/jira/**`
- `src/jirafs/export/**`
- matching tests

Implement:

- issue fetch
- issue search
- board and sprint reads needed for scope refresh
- current-user lookup when needed
- shallow export normalization

Depends on:

- P1
- P3
- P5

Acceptance criteria:

- one issue can be exported into the canonical local model
- one mirror scope such as `my-issues` or `current-sprint` can refresh
- linked issues are referenced shallowly, not recursively imported

## Packet P7: Planner

Own:

- `src/jirafs/plan/**`
- matching tests

Implement:

- local-vs-remote diffing
- conflict detection
- typed plan output

Depends on:

- P3
- P5
- P6

Acceptance criteria:

- no-op plans stay empty
- editable field changes become typed operations
- stale remote state becomes conflicts instead of silent overwrite

## Packet P8: Sync Applier

Own:

- `src/jirafs/sync/**`
- matching tests

Implement:

- apply validated plans
- refresh local metadata after success
- reject archive paths and unresolved refs before mutation

Depends on:

- P7

Acceptance criteria:

- sync only acts through validated plan shapes
- stale-state and invalid-transition failures happen before Jira mutation

## Packet P9: CLI Project Selection And Mirror Commands

Own:

- `src/jirafs/cli/**`
- `tests/cli/**`

Implement:

- `jirafs use`
- `jirafs mirror refresh`
- `jirafs mirror archive-sweep`
- prompt and non-interactive selection behavior

Depends on:

- P1
- P2
- P6
- P7

Acceptance criteria:

- interactive unresolved project selection can prompt
- non-interactive unresolved selection fails clearly
- mirror commands use project context consistently

## Packet P10: Archive And Board Follow-On

Own:

- `src/jirafs/archive/**`
- `src/jirafs/board/**`
- matching tests

Implement:

- archive movement and snapshot handling
- board projections from local mirror data

Depends on:

- P2
- P6
- P8
- P9

Acceptance criteria:

- archive sweep preserves snapshots and removes live membership only when eligible
- board views can group by status, assignee, and epic

## Parallel Start Set

The safest initial parallel coding set is:

- P0 settings skeleton
- P3 canonical issue and registry models
- P4 markdown codec once P3 contracts are frozen

The next parallel wave is:

- P1 context resolver
- P5 registry and reference resolution
- P6 Jira read path

The final core wave is:

- P2 mirror scope model
- P7 planner
- P8 sync applier
- P9 CLI project selection and mirror commands

P10 should stay behind the core read/plan/sync milestone.
