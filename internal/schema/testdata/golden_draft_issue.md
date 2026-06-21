---
key: 'DRF-1'
type: 'story'
project: 'project:DRF'
schema_version: '1'
state: 'draft'
---
## Description
This task creates a golden fixture for the draft issue round-trip test.

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
- Draft state (no remote metadata)

## Comments To Add

## Remote Comments
Golden fixture created for WS1 schema/codec task B033b.

