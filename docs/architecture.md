# Architecture

## Goal

`jirafs` is a standalone CLI-first project that turns Jira into a local,
structured workspace while preserving safe round-trip sync.

The system must support:

- local editing of current and future issues
- local creation of new issues
- reliable sync back to Jira
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

## Filesystem Model

Suggested project layout:

```text
jirafs/
  README.md
  docs/
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

### Live vs Archive

- `jira/live/issues/` holds syncable issues mirrored from Jira.
- `jira/live/drafts/` holds locally created issues not yet uploaded.
- `jira/archive/` holds immutable or mostly immutable historical snapshots.

This split prevents accidental edits to old issues from being pushed back into
Jira while still preserving a rich historical corpus.

## Core Library Responsibilities

The library should be split into modules with clear ownership:

- schema
- markdown codec
- registry loader
- reference resolver
- Jira client
- sync planner
- sync applier
- template engine
- board/query projections

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

### Registry Loader and Resolver

Owns typed references for:

- users
- sprints
- fix versions
- issues
- epics
- projects

The resolver must convert local refs into Jira API identifiers during sync.

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
- registry files own reference metadata
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

1. export one issue to Markdown
2. import one issue document
3. create one new issue from a template
4. sync a limited set of editable fields
5. append comments
6. resolve typed references for users, fix versions, sprints, and epics

That is enough to validate the model before board views and larger archive
features are built.
