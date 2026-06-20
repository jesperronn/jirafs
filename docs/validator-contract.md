# Validator Contract

## Goal

Every edit must be followed by explicit validation.

This project uses structured Markdown documents and code modules that act as
contracts. A change is incomplete until the relevant validator checks pass and
the evidence is recorded.

Read this together with:

- [Verification Policy](verification-policy.md)
- [Development Rules](development-rules.md)
- [Issue Format](issue-format.md)
- [Sync Model](sync-model.md)

## Validation Phases

Validation happens at three levels:

### Edit-Level Validation

Run immediately after a local change to confirm the edited surface is still
well-formed.

Examples:

- rerun the targeted unit test
- rerun the Markdown or schema validator
- rerun the relevant formatter or autofixer check
- rerun the round-trip fixture for a changed document codec

### Task-Level Validation

Run when the assigned task is complete.

Examples:

- related tests for the touched module
- `bin/lint`
- the local full test wrapper when required by policy

### Integration-Level Validation

Run when multiple builder outputs or multiple modules come together.

Examples:

- full suite
- coverage gate
- cross-module integration tests
- manual CLI and filesystem verification for user-visible workflows

## Structured Markdown Validation

When a task edits structured Markdown or Markdown-derived contracts, the
validator must check at least:

- required frontmatter keys
- section names and section ownership rules
- typed reference shape
- parse and render round-trip stability where applicable
- broken internal document links

For document-shape work, "the Markdown looks right" is not sufficient.

## Post-Edit Rule

After any edit, the responsible worker must answer:

1. What validator applies to this edit?
2. Was it run immediately after the change?
3. What exact command proved it?
4. Did the proof cover the final diff?

If any answer is missing, the edit is not complete.

## Validator Output Expectations

Validator output should be:

- deterministic
- scoped enough for fast local loops
- clear about the failing file or contract
- usable by both humans and simple agents

Prefer validator output that names:

- the file
- the violated rule
- the expected shape
- the actual failing condition

## Builder Reporting Requirement

Every builder handoff must include a validation block with:

- exact command lines
- pass or fail result for each command
- whether the command was edit-level, task-level, or integration-level
- any required checks still missing

This is required even for documentation-only tasks when the repo has document
validators.

## Failure Policy

If a validator fails:

1. stop expanding scope
2. fix the smallest real cause
3. rerun the failing validator
4. rerun the related local validator set
5. update the evidence to reflect the final passing state

Do not hand off work with stale validation evidence from an earlier revision.

## Standard Validator Layers For This Repo

At the current docs-first stage, the minimum validator stack is:

- repo document checks from `tools/lint_repo.py`
- repository lint wrapper via `bin/lint`
- repository test and coverage wrapper via `bin/test`

As implementation grows, add targeted validators for:

- Go formatting and linting
- schema validation
- codec round-trip fixtures
- resolver contract tests
- planner and sync integration tests

## Default Standard

- every edit has a validator
- validators run immediately after relevant edits
- task completion requires recorded evidence
- integration requires broader validation than local editing
