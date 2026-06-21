# Pre-Live Parallel Plan

## Goal

This document turns the remaining MVP work into worker-sized task streams that
can run with 2 or 3 parallel agents while preserving path ownership.

Target outcome:

- export one real Jira issue into the canonical local format
- refresh one shallow live mirror scope
- edit one exported issue locally
- plan safe Jira mutations for a narrow field set
- sync those mutations back to Jira
- re-export and confirm the new synced baseline

This is the minimum pre-live tranche. Stretch goals stay out of the critical
path until that loop is green against real data.

## Release Gates

The pre-live tranche is complete only when all of the following are true:

- credential refs resolve for at least `env://` and `file://`
- `jirafs export issue ABC-123` writes a canonical Markdown issue file
- `jirafs mirror refresh my-issues` or `current-sprint` refreshes a live set
- `jirafs plan` emits typed operations and typed conflicts
- `jirafs sync` applies one validated update path safely
- one real-data smoke test has been run against a disposable Jira issue
- `bin/test` and `bin/lint` pass at the integration point

## Worker Layout

### Two Workers

Worker A:

- WS1 schema and codec
- WS4 plan and sync

Worker B:

- WS2 settings, context, credentials, registry, references
- WS3 Jira and export
- WS5 mirror, CLI, archive smoke path

Use this mode when only two loops are available. Worker A owns the contract
and write path; Worker B owns live Jira access and operator flow.

### Three Workers

Worker A:

- WS1 schema and codec

Worker B:

- WS2 settings, context, credentials, registry, references
- WS3 Jira and export

Worker C:

- WS4 plan and sync
- WS5 mirror, CLI, archive smoke path

Use this mode when three loops are available. This is the preferred split for
the pre-live tranche.

## Stream Order

These streams can overlap, but the dependency edges matter.

### Wave 1

- WS1 finishes issue body parsing and deterministic rendering.
- WS2 adds credential resolution.
- WS3 adds authenticated live client seams and real search support.

### Wave 2

- WS3 writes single-issue export through the codec.
- WS5 wires `jirafs export issue` and `jirafs registry refresh`.
- WS5 wires `jirafs mirror refresh` for one named scope.

### Wave 3

- WS4 implements no-op and field-update planning.
- WS4 implements stale-state conflict detection.
- WS4 implements one safe sync execution path.

### Wave 4

- WS5 wires `jirafs plan` and `jirafs sync`.
- WS5 adds a disposable real-data smoke path.
- Integration verifies one end-to-end round trip.

## Stream Backlogs

### WS1 Schema And Codec

Objective:
close the document contract so export, plan, and sync stop depending on stubs.

Ready queue:

- B031a split issue body into ordered `##` sections
- B031b populate canonical sections including empty ones
- B031c reject unknown headings explicitly
- B032a render frontmatter in stable field order
- B032b render fixed sections in stable order
- B033a add synced issue round-trip golden fixture
- B033b add draft issue round-trip golden fixture

Exit condition:

- issue files round-trip deterministically on second render

### WS2 Settings, Context, Credentials, References

Objective:
make one real Jira project usable without hard-coded secrets.

Ready queue:

- B017a parse `credential_refs` without leaking provider behavior into callers
- B017b resolve `env://VAR_NAME`
- B017c resolve `file://path`
- B017d merge ordered credential sources with later override
- B017e validate required credential payload by `auth_type`
- B017f expose resolved instance auth config to Jira client callers

Exit condition:

- one configured project can resolve live auth material locally

### WS3 Jira And Export

Objective:
complete the read side for one real issue and one shallow mirror scope.

Ready queue:

- B054a build authenticated request headers from resolved credential payload
- B054b add current-user lookup when scope resolution needs it
- B055a implement real `SearchIssues` for `my-issues`
- B055b implement real `SearchIssues` for `current-sprint`
- B056a export one fetched issue through the canonical codec
- B056b refresh project-scoped registries needed by exported refs
- B056c keep linked issues shallow during export and mirror refresh

Exit condition:

- one issue export and one named scope refresh work against real Jira data

### WS4 Plan And Sync

Objective:
make the first safe write loop real.

Ready queue:

- B060a no-op plan for unchanged summary and description
- B060b no-op plan for unchanged refs and sync metadata
- B061a typed operations for summary and description edits
- B061b typed operations for labels, assignee, status, sprint, fix versions
- B062a stale remote version becomes conflict
- B062b stale content hash becomes conflict
- B063a sync accepts validated no-op plan without mutation
- B063b sync applies one validated field-change operation
- B064a archive paths and unresolved refs fail before mutation
- B064b stale state and invalid transitions fail before mutation

Exit condition:

- one edited issue can plan and safely sync a narrow field set

### WS5 Mirror, CLI, Archive, Smoke

Objective:
expose the core loop through the binary and prove it on real data.

Ready queue:

- B070a model explicit issue imports with explainable membership reason
- B070b model named scope membership alongside explicit imports
- B071a out-of-scope resolved issues become archive-eligible
- B071b pinned or unsynced issues remain live
- B080a `jirafs use --project` updates remembered project
- B080b `jirafs use` interactive and non-interactive behavior matches docs
- B081a `jirafs mirror refresh` resolves project context and calls refresh service
- B081b `jirafs mirror refresh` reports deterministic changed issue keys
- B082a `jirafs mirror archive-sweep` reports eligible actions without mutation
- B082b `jirafs mirror archive-sweep --apply` calls archive service
- B083a wire `jirafs export issue`
- B083b wire `jirafs plan`
- B083c wire `jirafs sync`
- B083d add disposable real-data smoke script for export-edit-plan-sync-reexport

Exit condition:

- the binary can drive the first end-to-end real-data loop

## Integration Order

Integrate in this order to keep rebase churn low:

1. WS1 codec completion
2. WS2 credential resolution
3. WS3 authenticated read path and export
4. WS5 export and mirror CLI wiring
5. WS4 planner
6. WS4 sync applier
7. WS5 plan/sync CLI wiring and smoke script

## First Stretch Goals

These should wait until the pre-live tranche is green.

- template-based draft creation
- bounded backlog and next-sprint mirror scopes
- archive movement and historical snapshots
- local board views by status, assignee, and epic
- batch export by sprint, backlog, and JQL
- archive-driven analytics and agent workflows
