# Templates

## Goal

Templates should support local-first creation of new issues without reducing
them to plain text snippets.

Templates must be schema-aware so they can prefill:

- issue type
- default labels
- required sections
- default custom fields
- required references
- readiness checklists

## Core Templates

The initial template set should include:

- story
- bug
- epic

## Storage

Suggested path:

```text
jira/templates/
  story.md
  bug.md
  epic.md
```

Project-specific overrides can be added later:

- `jira/templates/project-abc/story.md`

## Template Shape

Templates should use the same document format as normal issue files, but with
placeholder values and `sync.mode: create`.

Example traits for each template:

### Story

- `workflow.issue_type: Story`
- sections for Description and Acceptance Criteria
- optional Definition of Ready section
- default labels for product or team conventions

### Bug

- `workflow.issue_type: Bug`
- sections for Description, Expected Behavior, Actual Behavior, Reproduction,
  and Impact
- optional severity custom field

### Epic

- `workflow.issue_type: Epic`
- sections for Goal, Scope, Non-Goals, Success Metrics
- support downstream issue linkage

## Required Field Validation

Templates should declare required fields or sections so draft validation can
fail before Jira sync.

Validation examples:

- summary must be non-empty
- issue type must be set
- project ref must resolve
- bug template requires Reproduction or equivalent content

## Template Selection

Creation paths should support:

- `jirafs new story`
- `jirafs new bug`
- `jirafs new epic`
- `jirafs new --template path/to/custom-template.md`

## Agent Use

Agents should use templates as a controlled target surface when generating
drafts from unstructured chat or notes.

That keeps agent output aligned with the same constraints used by the CLI.
