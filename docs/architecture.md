# Architecture

## Goal

`jirafs` is a standalone CLI-first project that turns Jira into a local,
structured workspace while preserving safe round-trip sync.

The system must support:

- local editing of current and future issues
- local creation of new issues
- reliable sync back to Jira
- selective live mirroring of one or more Jira working scopes
- typed references to related Jira entities
- long-lived archival data for later analysis and agent use

## Product Shape

The product has four layers:

1. Filesystem model
2. Core library
3. CLI
4. Agent integration

The CLI is the primary public interface. Agents and skills should orchestrate
the CLI or reuse the same library rather than owning Jira logic themselves.

## Implementation Language

The implementation target is Go.

That choice is driven by distribution and maintenance concerns more than raw
prototyping speed:

- `jirafs` should be installable as a single binary
- operators should not need to manage an interpreter or virtual environment
- release packaging should stay simple across machines
- the codebase should avoid runtime dependency drift

The architectural module split remains the same regardless of language.

## Filesystem Model

Suggested filesystem layout:

```text
~/.jirafs/
  settings.toml
  state/
    current.json

<project-mirror-dir>/
  jira/
    live/
      issues/
      drafts/
    archive/
      sprints/
      issues/
    registry/
      users.yaml
      sprints.yaml
      fix_versions.yaml
      projects.yaml
      issue_types.yaml
      statuses.yaml
    templates/
      story.md
      bug.md
      epic.md
    config.yaml
```

The mirror directory is intentionally independent of the code repository that
may be associated with a Jira project.

### Live vs Archive

- `jira/live/issues/` holds syncable issues mirrored from Jira.
- `jira/live/drafts/` holds locally created issues not yet uploaded.
- `jira/archive/` holds immutable or mostly immutable historical snapshots.

This split prevents accidental edits to old issues from being pushed back into
Jira while still preserving a rich historical corpus.

### Membership-Driven Live Mirror

The live mirror is membership-driven rather than a full Jira dump.

An issue belongs in the live mirror because:

- it was explicitly imported by key
- it currently matches one or more named mirror scopes
- it is a local draft with pending sync

Named scopes such as `current-sprint`, `my-issues`, `backlog`, and
`next-sprint` are part of the local model and are refreshed against Jira state.

## Core Library Responsibilities

The library should be split into modules with clear ownership:

- schema
- markdown codec
- settings loader
- context resolver
- registry loader
- reference resolver
- Jira client
- mirror manager
- sync planner
- sync applier
- template engine
- board/query projections

## Dependency Constraints

The implementation should stay conservative about dependencies.

Rules:

- prefer the standard library first
- add third-party packages only for real leverage, not convenience alone
- prefer narrow packages with small transitive graphs
- avoid framework-heavy libraries for CLI, config, or HTTP work
- keep static builds and cross-platform distribution easy

This matters especially for:

- TOML and YAML parsing
- Markdown and frontmatter handling
- CLI command routing
- HTTP client behavior

Each non-trivial dependency should justify:

- why the standard library is insufficient
- why the package is safer or simpler than a local implementation
- what operational or maintenance cost it adds

### Schema

Owns the canonical document model:

- issue document
- registry entries
- template document
- sync plan
- operation payloads

### Markdown Codec

Owns round-tripping between:

- local Markdown files
- normalized in-memory issue documents

The codec must be deterministic and preserve stable ordering for machine-owned
fields so diffs remain readable.

### Settings Loader and Context Resolver

These modules own:

- `~/.jirafs/settings.toml`
- Jira instance definitions
- project definitions
- mirror directory selection
- current-project memory
- effective user resolution
- current working directory project detection

They provide the active project context for CLI commands.

### Registry Loader and Resolver

Owns typed references for:

- users
- sprints
- fix versions
- issues
- epics
- projects

The resolver must convert local refs into Jira API identifiers during sync.

### Mirror Manager

The mirror manager owns:

- named mirror scopes
- mirror membership state
- refresh behavior
- archive sweep rules
- movement between live and archive sets

### Jira Client

Owns all Jira HTTP interactions:

- issue fetch
- issue search
- issue create
- issue update
- comment append
- transition lookup and apply
- Agile board and sprint queries
- metadata and registry refresh
- current-user lookup for `me`-style scopes when needed

### Sync Planner

Owns comparison between:

- remote issue state
- local issue state
- last-synced state

The planner produces explicit operations and conflicts instead of mutating
Jira directly during comparison.

### Sync Applier

Owns safe write execution:

- create issue
- update fields
- append comments
- link issues
- assign epic
- update fix versions
- set sprint
- run transitions

## CLI

The CLI should remain scriptable and explicit. Core commands are documented in
[CLI](cli.md).

The CLI should expose plan-first workflows:

- use
- mirror
- export
- plan
- sync
- new
- board

## Agent Integration

Agent workflows are optional and sit above the CLI:

- generate issue drafts from chat or notes
- suggest field values and templates
- summarize sync plans
- analyze archives and propose process improvements

The core project must remain useful without any agent runtime.

## Data Ownership

Ownership rules:

- frontmatter fields are the canonical structured layer
- fixed Markdown sections are the canonical rich-text layer
- global settings own Jira instances, projects, and current context selection
- registry files own reference metadata
- mirror state owns scope membership and archive eligibility
- sync metadata owns remote identity and conflict checks

No implicit inference should be required for critical write paths.

## Non-Goals For MVP

- full Jira feature parity
- arbitrary custom workflow authoring
- WYSIWYG rich text editing
- complete comment editing history sync
- attachment upload/download
- multi-backend support beyond Jira

## MVP

The first usable version should support:

1. load global settings from `~/.jirafs/settings.toml`
2. resolve one active project from flags, cwd, or remembered state
3. export one issue to Markdown
4. refresh one shallow live mirror scope
5. create one new issue from a template
6. sync a limited set of editable fields
7. append comments
8. resolve typed references for users, fix versions, sprints, and epics

That is enough to validate the model before board views and larger archive
features are built.
