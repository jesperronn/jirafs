# References

## Goal

References to Jira entities must be typed and stable.

Plain display strings are not sufficient for safe sync because names can change
and different Jira APIs often require opaque identifiers.

## Typed Reference Syntax

Recommended reference forms:

- `user:jesper`
- `issue:ABC-123`
- `version:abc-1.4.0`
- `sprint:platform-2026-w25`
- `project:abc`
- `issuetype:story`

These refs appear inside issue frontmatter and are resolved through registry
files.

## Registry Files

Suggested files:

- `jira/registry/users.yaml`
- `jira/registry/sprints.yaml`
- `jira/registry/fix_versions.yaml`
- `jira/registry/projects.yaml`
- `jira/registry/issue_types.yaml`
- `jira/registry/statuses.yaml`

## User Registry

Example:

```yaml
users:
  user:jesper:
    account_id: "712020:abcd"
    display_name: "Jesper Ronn"
    email: "jesper@example.com"
    active: true
```

## Sprint Registry

Example:

```yaml
sprints:
  sprint:platform-2026-w25:
    id: 482
    board_id: 17
    name: "Platform Sprint 2026 W25"
    state: active
    start_date: 2026-06-15
    end_date: 2026-06-28
```

## Fix Version Registry

Example:

```yaml
fix_versions:
  version:abc-1.4.0:
    id: "19384"
    project_key: ABC
    name: "1.4.0"
    released: false
```

## Epic and Issue References

Epics should be treated as issues with special semantics.

Structured issue relations should include:

- `parent`
- `epic`
- `links.blocks`
- `links.blocked_by`
- `links.relates_to`
- `links.duplicates`

Using explicit fields for parent and epic is better than forcing everything
into a generic link model.

## Snapshot Display Values

For archival clarity, issue files may store display snapshots alongside refs.

Example:

```yaml
snapshot:
  sprint_names:
    - "Platform Sprint 2026 W25"
  fix_version_names:
    - "1.4.0"
```

This preserves historical readability even if Jira entities are renamed later.

## Resolution Rules

Resolution should be deterministic:

1. Load registry.
2. Resolve typed refs to Jira identifiers.
3. Fail planning if a required ref is missing or ambiguous.

The first implementation should avoid fuzzy matching during sync.

## Jira-Specific Notes

- users often need `accountId`, not email
- sprints often need Agile API ids
- fix versions need project-scoped version ids
- epic assignment may vary by Jira setup and field configuration

The resolver layer must isolate those Jira-specific details from the document
format.
