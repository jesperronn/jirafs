---
agent: pi
credit: false
commands:
  - name: git_status
    run: git status --short
  - name: stream_ledger
    run: cat docs/ralph-stream-ws1-schema-codec.md
  - name: recent_commits
    run: git log --oneline -3
  - name: tests
    run: bin/test
  - name: lint
    run: bin/lint
---
# jirafs WS1 Ralph Loop

Implement one WS1 schema/codec task, then stop.

Pick the first unchecked task whose deps are checked in the stream ledger.

Rules:

- Stay inside `internal/schema/**` and `internal/codec/**`.
- Add tests with new code.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after both gates pass.
- Commit implementation, tests, and ledger checkbox in one conventional commit.
- Do not commit blocked or failing work.
- Handoff must include final gate results and commit hash.
- Read only the docs needed for the chosen task.

After committing, run `bin/integrate_stream_commit`.

## Current Git Status

{{ commands.git_status }}

## Stream Ledger

{{ commands.stream_ledger }}

## Recent Commits

{{ commands.recent_commits }}

## Current Test Output

{{ commands.tests }}

## Current Lint Output

{{ commands.lint }}

## Required Handoff

```text
Stream:
- WS1 schema-codec

Task:
- <id and objective>

Validation:
- bin/test: <pass/fail>
- bin/lint: <pass/fail>
- bin/integrate_stream_commit: <pass/fail>

Files changed:
- <paths>

Commit:
- <hash and subject, or none if partial/blocked>

Status:
- <done|partial|blocked>

Risks:
- <open issues or none>
```
