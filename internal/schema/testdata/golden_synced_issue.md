---
key: 'WS1-42'
type: 'story'
project: 'project:WS1'
schema_version: '1'
state: 'synced'
remote_version: '7'
content_hash: 'a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6'
sync_time: '2026-06-21T14:30:00Z'
---
## Description
This task creates a golden fixture for the synced issue round-trip test.

The fixture exercises all frontmatter fields and every fixed section.

## Acceptance Criteria
- Fixture exists as a standalone file under testdata/
- ParseIssue renders identical output
- Round-trip preserves all fields and section content
- No fields are lost or reordered

## Definition of Ready
- Schema design approved
- Parse and render tested independently

## Notes
This fixture covers:
- Identity fields (key, type, project)
- Machine-owned fields (schema_version)
- Remote metadata (state, remote_version, content_hash, sync_time)
- Editable fields (summary, labels, assignee, linked_issues)
- All six fixed sections

## Comments To Add

## Remote Comments
Golden fixture created for WS1 schema/codec task B033a.

