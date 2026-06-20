# Implementation Roadmap

## Current Baseline

The repository currently contains architecture documents but no implementation
tree. This roadmap assumes a greenfield first build and treats the existing
docs as the source of truth for:

- filesystem model
- issue document contract
- typed reference model
- plan-first sync flow
- CLI command surface

Implementation should proceed by freezing contracts before building features on
top of them. The main dependency chain is:

1. settings, context, and mirror contracts
2. document and registry contracts
3. codec and resolver
4. Jira read path
5. planner
6. sync applier
7. draft/template flow
8. bulk export, archive, and board projections

This roadmap should be read together with:

- [Parallel Workstreams](parallel-workstreams.md)
- [Verification Policy](verification-policy.md)
- [Mirror Model](mirror-model.md)
- [Settings And Context](settings-and-context.md)
- [Project Selection CLI](project-selection-cli.md)

## Architectural Checkpoints

These checkpoints gate work across milestones. Do not bypass them.

### Checkpoint 0: Settings And Context Contract Is Frozen

Freeze the operator-level configuration and project resolution rules first.

Exit condition:

- `~/.jirafs/settings.toml` shape is fixed for the first implementation
- project resolution precedence is fixed
- mirror directory ownership is fixed
- current-project memory rules are fixed
- commands know when to prompt versus fail

### Checkpoint A: Document Contract Is Frozen

Freeze the canonical in-memory models and the Markdown/frontmatter layout for:

- issue documents
- registry documents
- sync plan documents
- operation payloads

Exit condition:

- one canonical schema definition exists for each document family
- field ownership is explicit: editable, append-only, read-only
- section names are fixed and tested
- the roadmap’s later milestones can depend on stable parse/render behavior

### Checkpoint B: Resolver Contract Is Frozen

Freeze how typed refs are represented and resolved.

Exit condition:

- registry file names and shapes are fixed
- resolver inputs and outputs are fixed
- missing and ambiguous reference failures are structured
- Jira-specific ids stay behind the resolver boundary

### Checkpoint C: Plan Format Is Frozen

Freeze the shape of a sync plan before implementing write execution.

Exit condition:

- each write operation has a typed plan representation
- conflicts are first-class plan items, not free-form strings
- `plan` output is stable enough for both humans and JSON consumers

### Checkpoint D: Sync Safety Rules Are Enforced

Do not expose general sync until conflict detection, stale-state checks, and
non-syncable archive protection are implemented.

Exit condition:

- remote version and content-hash checks are active
- archive paths are rejected by sync
- invalid transitions and unresolved refs fail during planning
- sync only applies a previously validated plan shape

## Milestones

### Milestone 0: Project Skeleton and Test Harness

Objective:
Establish a working implementation layout, packaging choice, CLI entrypoint,
and test harness before feature code starts to spread.

Deliverables:

- source tree for the chosen language runtime
- CLI bootstrap with subcommand registration
- fixture directories for issue docs, registries, and mocked Jira payloads
- test runners for unit, golden-file, and CLI integration tests

Sequencing:

1. Choose the runtime and package layout.
2. Create CLI entrypoint with placeholder commands matching the documented
   command families.
3. Set up fixture loading and golden-file assertions.

Acceptance criteria:

- the repo can run a test suite locally
- the CLI prints help for `init`, `export`, `plan`, `sync`, `new`, `registry`,
  `board`, and `archive`
- fixture-based tests can compare exact Markdown output
- lint, tests, and coverage commands exist even if some checks are initially
  expected to fail before implementation begins

Parallel work:

- Allowed: CLI bootstrap, test harness, and fixture authoring can proceed in
  parallel once the package layout is chosen.
- Forbidden: feature modules must not invent their own ad hoc test harnesses or
  command dispatch patterns.

### Milestone 0.5: Settings, Context, And Mirror Bootstrap

Objective:
Establish the operator-level configuration model before issue export and sync
work begins.

Deliverables:

- settings loader for `~/.jirafs/settings.toml`
- project and instance model validation
- context resolver for `--project`, cwd detection, and remembered state
- initial mirror scope model and mirror membership state

Sequencing:

1. Implement settings parsing and validation.
2. Implement project resolution precedence.
3. Implement current-project memory read and write.
4. Implement mirror scope definitions and mirror directory resolution.

Architectural checkpoint:

- Must satisfy Checkpoint 0 before Milestone 4 begins.

Acceptance criteria:

- multiple Jira instances and projects can be configured
- one project can point at a mirror directory outside any code repo
- cwd-based project detection works from configured local folders
- non-interactive unresolved project selection fails clearly
- interactive unresolved project selection can prompt from known projects

Parallel work:

- Allowed: settings parsing and CLI project-selection UX can proceed in
  parallel once one shared context model is agreed.
- Forbidden: export, board, or mirror-refresh code must not invent private
  project resolution rules.

### Milestone 1: Canonical Models and Schema Enforcement

Objective:
Turn the document contracts from the docs into enforceable code-level models.

Deliverables:

- issue document model
- registry models
- sync metadata model
- sync plan and operation models
- validation errors with machine-readable codes

Sequencing:

1. Define the in-memory issue model directly from `docs/issue-format.md`.
2. Define registry models from `docs/references.md`.
3. Define sync plan and operation types from `docs/sync-model.md`.
4. Add validation for required fields, allowed sections, and ownership rules.

Architectural checkpoint:

- Must satisfy Checkpoint A before Milestone 2 starts.

Acceptance criteria:

- invalid frontmatter, unknown sections, and missing required sync metadata are
  rejected with structured validation errors
- typed refs are represented consistently across issue and registry models
- plan operations are modeled without depending on Jira HTTP code

Parallel work:

- Allowed: issue model and registry model implementation can proceed in
  parallel if one shared ref type module is agreed first.
- Forbidden: planner, template engine, and Jira client work must not start from
  private model drafts.

### Milestone 2: Markdown Codec and Golden Files

Objective:
Implement deterministic round-trip parsing and rendering for issue documents.

Deliverables:

- Markdown parser for frontmatter plus fixed sections
- deterministic renderer with stable field ordering
- golden fixtures for synced issues and draft issues

Sequencing:

1. Parse frontmatter into the Milestone 1 models.
2. Parse fixed sections into canonical rich-text fields.
3. Render normalized documents back to Markdown.
4. Add round-trip and formatting stability tests.

Architectural checkpoint:

- This milestone closes Checkpoint A in executable form.

Acceptance criteria:

- parsing then rendering a valid issue fixture is stable on the second render
- section ordering and machine-owned field ordering are deterministic
- read-only mirrored content is preserved but not treated as editable input
- draft files with `sync.mode: create` and synced files with remote ids both
  round-trip correctly

Parallel work:

- Allowed: parser and renderer can be split if both target the same canonical
  model and fixture set.
- Forbidden: export and template work must not write Markdown directly; they
  must wait for the codec.

### Milestone 3: Registry Loader and Reference Resolver

Objective:
Implement typed registry loading and deterministic reference resolution.

Deliverables:

- registry file loader
- resolver for users, sprints, versions, projects, issue types, and statuses
- structured missing-ref and ambiguous-ref errors

Sequencing:

1. Implement registry parsing and validation.
2. Implement ref lookup by typed key.
3. Map local refs to Jira identifiers without exposing Jira-specific id rules to
   callers.
4. Add resolver tests for renamed display values and missing entries.

Architectural checkpoint:

- Must satisfy Checkpoint B before planner or sync writes depend on refs.

Acceptance criteria:

- all documented registry files load from the expected paths
- resolver returns Jira ids and display metadata from typed refs
- missing refs fail before any plan is considered executable
- no fuzzy matching is performed

Parallel work:

- Allowed: separate registry families can be implemented in parallel after the
  common loader contract is frozen.
- Forbidden: Jira client update code must not embed direct account-id or sprint
  lookup logic outside the resolver layer.

### Milestone 4: Jira Read Path and Export

Objective:
Implement the remote read side first, so local files are grounded in real Jira
payloads before any write path exists.

Deliverables:

- Jira client read operations for issue fetch, issue search, sprint/board
  queries, and metadata refresh
- export command for a single issue first
- registry refresh command for the initial registry set
- mirror refresh for at least one named scope such as `my-issues` or
  `current-sprint`

Sequencing:

1. Build authenticated Jira client read primitives.
2. Implement remote-to-model normalization.
3. Write exported issue docs through the codec.
4. Write registry refresh outputs through registry models.
5. Add CLI integration for `export issue` and `registry refresh`.
6. Add live mirror refresh for one shallow scope.

Acceptance criteria:

- exporting one remote issue produces a valid local issue file under
  `jira/live/issues/`
- export populates sync metadata required for later conflict detection
- registry refresh writes valid registry files for the supported entity types
- re-exporting the same unchanged issue produces stable local output
- mirror refresh can populate a shallow live working set without recursively
  importing every linked issue

Parallel work:

- Allowed: issue export and registry refresh can proceed in parallel after the
  Jira read primitives and normalization contract exist.
- Forbidden: no sync or draft-upload code should start before at least one
  exported issue fixture exists and passes round-trip tests.

### Milestone 5: Planner and Conflict Engine

Objective:
Implement plan-first comparison across local state, remote state, and the last
synced baseline.

Deliverables:

- sync planner
- conflict detector
- human-readable and JSON plan output

Sequencing:

1. Normalize exported remote state into the canonical issue model.
2. Compare local editable fields against last-synced state.
3. Compare current remote state against last-synced state.
4. Emit explicit operations and explicit conflicts.
5. Surface plan output through `jirafs plan`.

Architectural checkpoint:

- Must satisfy Checkpoint C before sync application starts.

Acceptance criteria:

- unchanged files produce an empty or no-op plan
- supported local edits produce typed operations only for editable fields
- remote changes in untouched fields do not create false conflicts
- overlapping local and remote edits to the same field produce conflicts
- unresolved refs and invalid transitions surface during planning

Parallel work:

- Allowed: conflict classification and plan rendering can proceed in parallel
  once the core operation model is frozen.
- Forbidden: sync write execution must not duplicate planner logic or infer its
  own operations independently.

### Milestone 6: Safe Sync Applier for the MVP Field Set

Objective:
Apply validated plans back to Jira for the intentionally small initial field
set.

Deliverables:

- sync applier for create issue, field update, comment append, selected links,
  sprint assignment, fix versions, parent/epic assignment, and transitions
- post-apply local metadata refresh
- `jirafs sync` CLI path that consumes the planner output

Sequencing:

1. Reuse the frozen plan model from Milestone 5.
2. Implement the smallest supported operation set first.
3. Refresh local metadata after successful remote writes.
4. Reject archive paths and stale-state writes before the first API mutation.

Architectural checkpoint:

- Must satisfy Checkpoint D before `sync` is considered generally usable.

Acceptance criteria:

- `sync` never mutates Jira without going through a validated plan shape
- stale remote versions fail cleanly before mutation
- invalid transitions and unresolved refs fail before mutation
- successful sync refreshes local issue metadata to a new synced baseline
- the supported MVP write set matches the contract in [Issue Format](issue-format.md)

Parallel work:

- Allowed: operation executors can be built in parallel after the shared plan
  contract and Jira client mutation surface are fixed.
- Forbidden: command-level code must not bypass the sync applier to call Jira
  mutations directly.

### Milestone 7: Draft Creation and Template Flow

Objective:
Support local-first issue creation using schema-aware templates.

Deliverables:

- template loader and validator
- `jirafs new` draft creation flow
- draft-to-create plan generation
- create-issue sync path for template-based drafts

Sequencing:

1. Implement template loading against the canonical issue model.
2. Create draft files in `jira/live/drafts/`.
3. Reuse planner and sync machinery for create operations.
4. Rename or rehome drafts after successful Jira creation if needed.

Acceptance criteria:

- story, bug, and epic templates produce valid draft issue files
- invalid drafts fail local validation before any Jira call
- created issues receive real remote ids and sync metadata after upload
- draft creation does not fork a second document format

Parallel work:

- Allowed: template authoring and template loader work can run in parallel once
  the issue schema is frozen.
- Forbidden: templates must not invent fields or section names outside the
  canonical document contract.

### Milestone 8: Bulk Export, Archive, and Board Projections

Objective:
Layer higher-level local workspace features on top of the stable core.

Deliverables:

- batch export flows
- archive snapshot commands
- board and sprint projections
- archive-safe query surfaces for later agent use

Sequencing:

1. Extend export from single issue to sprint, JQL, and backlog scopes.
2. Implement archive placement and archive protection rules.
3. Build board projections from local issue files rather than direct Jira-only
   rendering.
4. Add CLI output modes for board and archive summaries.

Acceptance criteria:

- batch export writes stable issue files and registry state
- archived content is excluded from default sync paths
- board views can group by at least status, assignee, and epic
- archive data remains readable without weakening sync safety for live issues

Parallel work:

- Allowed: archive and board work can proceed in parallel once the live issue
  model, export path, and resolver are stable.
- Forbidden: archive logic must not leak into the live sync path as special
  cases.

## Global Rules

- Freeze contracts before expanding dependents.
- Keep implementation path-local when parallel workers are active.
- Route all Jira writes through planning and sync application.
- Treat coverage, lint, and full-suite success as merge gates, not aspirations.
- Update the architecture docs when a milestone intentionally changes a public
  contract.

Sequencing:

1. Implement field update execution for the MVP editable set.
2. Implement append-only comment handling.
3. Implement transition lookup and apply rules.
4. Refresh remote state after successful sync and rewrite local metadata.
5. Block archive paths and stale plans.

Architectural checkpoint:

- Must satisfy Checkpoint D before sync is considered usable.

Acceptance criteria:

- `sync` applies only operations that the planner can emit
- after a successful sync, the local file contains updated sync metadata and a
  new canonical content hash
- comments are appended, not edited in place
- archive paths are ignored or rejected by default
- stale remote versions or invalid transitions stop sync before partial writes

Parallel work:

- Allowed: operation executors can be split by operation family after the plan
  schema and Jira client interfaces are fixed.
- Forbidden: no executor may mutate local files directly except through the
  canonical refresh-and-render path.

### Milestone 7: Draft Creation and Template Flow

Objective:
Enable local-first creation of new issues using the same document contract as
synced issues.

Deliverables:

- template document loading
- `jirafs new` draft creation
- draft validation for required create-time fields
- create-mode sync path

Sequencing:

1. Implement template loading from `jira/templates/`.
2. Create draft files in `jira/live/drafts/` with `sync.mode: create`.
3. Validate project, issue type, and required create fields before upload.
4. Reuse the planner/applier path to create the issue and rewrite the file with
   real remote identity.

Acceptance criteria:

- `jirafs new story` creates a valid draft document
- create-mode planning shows a create operation instead of update operations
- successful create sync rewrites the draft with real `issue_id`, `issue_key`,
  and sync metadata
- invalid templates fail locally before any Jira write

Parallel work:

- Allowed: template authoring and template engine code can proceed in parallel
  after the issue codec is stable.
- Forbidden: draft creation must not fork into a separate document format or
  bypass planner validation.

### Milestone 8: Bulk Export, Archive, and Board Projections

Objective:
Add scale-oriented workflows only after the single-issue loop is stable.

Deliverables:

- export by JQL, backlog, or sprint
- archive snapshot commands
- local board/query projections

Sequencing:

1. Extend export from one issue to bounded issue sets.
2. Implement archive placement rules and non-syncable snapshot metadata.
3. Build board projections from local issue documents rather than from ad hoc
   Jira queries.

Acceptance criteria:

- bulk export writes multiple valid issue documents with stable filenames
- archive commands place files under `jira/archive/` with sync disabled
- board views can group local issues by sprint, assignee, or epic without
  adding new persistence formats

Parallel work:

- Allowed: bulk export, archive commands, and board projection code can proceed
  in parallel once the single-issue export and issue model are stable.
- Forbidden: board views must not depend on write-path internals or mutate live
  issue docs.

## Recommended Workstream Split

This is the highest-leverage parallelization that still respects the
architecture.

### Workstream 1: Contracts and Codec

Own:

- models
- validators
- Markdown codec
- golden fixtures

Starts immediately and blocks most other work.

### Workstream 2: Registry and Resolver

Own:

- registry loaders
- typed refs
- resolution errors

Can start once Milestone 1 has a shared ref representation.

### Workstream 3: Jira Read Path

Own:

- Jira auth and read client
- issue normalization
- export
- registry refresh

Can start once Milestones 1 and 2 are stable enough to accept exported data.

### Workstream 4: Planner and Sync

Own:

- diff engine
- conflict engine
- plan rendering
- sync executors

Must wait for Milestones 2 through 4. This workstream should not start by
guessing document or resolver behavior.

### Workstream 5: Templates and Projections

Own:

- template flow
- bulk export UX
- archive commands
- board views

Should start only after the single-issue export and sync loop works end to end.

## Explicit Forbidden Shortcuts

Do not take these implementation shortcuts:

- do not let the CLI parse Markdown directly; keep parsing in the codec layer
- do not let Jira payload shapes leak past the client and normalization layers
- do not let sync execute ad hoc writes without a typed plan
- do not treat archive files as syncable by default
- do not allow fuzzy reference matching in the first implementation
- do not introduce separate document formats for drafts and synced issues

## Definition of MVP Complete

The MVP is complete when all of the following are true:

- one remote issue can be exported into a stable local Markdown document
- that document can be edited locally in the supported field set
- `jirafs plan` produces explicit operations and conflicts from local, remote,
  and last-synced state
- `jirafs sync` safely applies the approved operations and refreshes local sync
  metadata
- a new issue can be created locally from a template and uploaded through the
  same planner/applier model
- typed refs for users, sprints, fix versions, and epics resolve
  deterministically

## Notes

- This roadmap intentionally delays board views, archive analysis, and broader
  Jira parity until the single-issue round-trip is reliable.
- If implementation pressure forces a narrower slice, cut scope by reducing the
  initial editable field set, not by weakening planner or conflict guarantees.
