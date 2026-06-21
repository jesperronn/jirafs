---
agent: pi
credit: false
commands:
  - name: git_status
    run: git status --short
  - name: stream_ledger
    run: cat docs/ralph-stream-ws3-jira-export.md
  - name: recent_commits
    run: git log --oneline -3
  - name: tests
    run: bin/test
  - name: lint
    run: bin/lint
---
# jirafs WS3 Ralph Loop

Implement one WS3 Jira/export task, then stop.

Pick the first unchecked task whose deps are checked in the stream ledger. If
no unchecked tasks remain, report `complete` and exit non-zero so
`ralph run --stop-on-error` stops the loop. If unchecked tasks remain but no
WS3 task is ready, report `blocked` and exit non-zero so
`ralph run --stop-on-error` stops the loop.

Rules:

- Stay inside `internal/jira/**` and `internal/export/**`.
- Add tests with new code.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after both gates pass.
- Commit implementation, tests, and ledger checkbox in one conventional commit.
- Do not commit blocked or failing work.
- Do not keep looping after `complete` or `blocked`.
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
- WS3 jira-export

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
- <done|partial|blocked|complete>

Risks:
- <open issues or none>
```
