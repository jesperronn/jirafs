---
agent: opencode
credit: false
commands:
  - name: git_status
    run: git status --short
  - name: stream_ledger
    run: cat docs/ralph-stream-ws2-settings-references.md
---
# jirafs WS2 Ralph Loop

Implement one WS2 settings/context/registry/reference task, then stop.

Pick the first unchecked task whose deps are checked in the stream ledger.
If no unchecked tasks remain, report `complete` and exit non-zero so
`ralph run --stop-on-error` stops the loop. If unchecked tasks remain but none
are ready, report `blocked` and exit non-zero so `ralph run --stop-on-error`
stops the loop.

Rules:

- Stay inside `internal/config/**`, `internal/context/**`,
  `internal/registry/**`, and `internal/references/**`.
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

## Required Handoff

```text
Stream:
- WS2 settings-references

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
