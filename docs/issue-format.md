# Issue Format

## Goal

Issue files must be:

- readable by humans
- editable in a normal editor
- reversible into Jira operations
- stable enough for archival and agent use

## One File Per Issue

Each issue is stored as one Markdown file with YAML frontmatter and fixed
section headings.

Example path:

```text
jira/live/issues/ABC-123.md
```

## Frontmatter Contract

Frontmatter is the canonical structured layer.

Example:

```md
---
schema: jirafs/jira-issue-v1
remote:
  jira_base_url: https://jira.example.com
  issue_id: "481516"
  issue_key: ABC-123
sync:
  mode: update
  remote_version: 17
  exported_at: 2026-06-20T10:15:00Z
  last_synced_at: 2026-06-20T10:15:00Z
  content_hash: sha256:example
workflow:
  project: ABC
  issue_type: Story
  status: In Progress
relations:
  assignee: user:jesper
  reporter: user:alice
  parent: null
  epic: issue:ABC-1
  sprint:
    - sprint:platform-2026-w25
  fix_versions:
    - version:abc-1.4.0
  affects_versions: []
  labels:
    - release
    - cli
  components:
    - tooling
links:
  blocks: []
  blocked_by: []
  relates_to: []
  duplicates: []
fields:
  summary: Improve release note generation
permissions:
  editable:
    - summary
    - description
    - labels
    - fix_versions
    - assignee
    - parent
    - epic
    - sprint
  append_only:
    - comments
  read_only:
    - reporter
    - created
    - status
created: 2026-06-18T08:30:00Z
updated: 2026-06-20T09:12:33Z
---

# ABC-123 Improve release note generation

## Description

We need release-note generation to include Jira-linked metadata and preserve
issue ordering.

## Acceptance Criteria

- Show Jira summary
- Group by commit type
- Support dry-run preview

## Comments To Add

- Ready for QA after dry-run output is verified.

## Remote Comments

<!-- read-only -->
- 2026-06-19 alice:
  Please keep the old output format stable.
```

## Editable vs Read-Only Content

Not every displayed field should be editable.

Editable:

- summary
- description
- labels
- assignee
- parent
- epic
- fix versions
- sprint
- selected custom fields

Append-only:

- comments to add

Read-only mirror:

- status
- reporter
- created
- updated
- remote comments history
- remote identifiers

## Rich Text Sections

The importer should only recognize fixed section names.

Core sections:

- `Description`
- `Acceptance Criteria`
- `Definition of Ready`
- `Notes`
- `Comments To Add`
- `Remote Comments`

Project or instance-specific sections may later be mapped through config to
custom Jira fields.

## New Issues

Draft issues use the same format with `sync.mode: create`.

Example:

```yaml
remote:
  jira_base_url: https://jira.example.com
  issue_id: null
  issue_key: null
sync:
  mode: create
workflow:
  project: ABC
  issue_type: Bug
template: bug
```

After creation, the file is updated with its real `issue_id`, `issue_key`, and
sync metadata.

## Status and Transitions

Issue status should be mirrored, but not treated as a directly editable scalar.

Instead, write operations should use explicit transition requests through
operations in frontmatter or CLI flags.

## Comments

Remote comments are shown for context, but should not be edited in place.

Local comments should be stored in `Comments To Add` and treated as append-only
operations during sync.

## File Naming

Recommended file naming:

- synced issues: `ABC-123.md`
- drafts before upload: `draft-<slug>.md`

Draft files may be renamed after successful creation.
