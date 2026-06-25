---
agent: pi
credit: false
commands:
  - name: git_status
    run: git status --short
  - name: task_ledger
    run: cat docs/ralph-loop-implementation-tasks.md
  - name: recent_commits
    run: git log --oneline -3
  - name: tests
    run: bin/test
  - name: lint
    run: bin/lint
---
# jirafs Pi Implementation Loop

Implement one `jirafs` task, then stop.

Pick the first unchecked task whose deps are done in the ledger below.

Rules:

- One task per iteration. Do not start a second task.
- Stay inside owned paths except for explicitly named integration files.
- New code needs tests in the same task.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after both gates pass.
- Commit after gates pass.
- Do not commit blocked or failing work.
- After handoff, exit immediately.
- No unchecked tasks left: report `complete` and exit non-zero.
- Tasks remain but none ready: report `blocked` and exit non-zero.
- Handoff must include final gate results and commit hash.
- Read only the docs needed for the chosen task.

Progress source: the task ledger is canonical. Each iteration starts at the
first unchecked task whose deps are checked. Do not plan later work in the
handoff.

Project: Go CLI for local-first Jira Markdown workspace. Prefer stdlib; justify
new dependencies.

## Current Git Status

{{ commands.git_status }}

## Task Ledger

{{ commands.task_ledger }}

## Recent Commits

{{ commands.recent_commits }}

## Current Test Output

{{ commands.tests }}

## Current Lint Output

{{ commands.lint }}

## Required Handoff

Return this:

```text
Task:
- <id and objective>

Scope:
- <owned paths>

Acceptance:
- <what is now true>

Validation:
- bin/test: <pass/fail>
- bin/lint: <pass/fail>
- other: <targeted tests/manual checks>

Files changed:
- <paths>

Commit:
- <hash and subject, or none if partial/blocked>

Status:
- <done|partial|blocked>

Risks:
- <open issues or none>
```
