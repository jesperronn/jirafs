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
  - name: gates
    run: bin/test && bin/lint
  - name: integrate
    run: bin/integrate_stream_commit
---
# jirafs Pi development loop

Implement one task, then stop.

Pick the first unchecked task in 
`docs/ralph-loop-implementation-tasks.md` whose deps are checked.

Rules:

- One task per iteration. Do not start a second task.
- Stay inside the owned paths listed by the chosen task.
- Prefer tests-only or narrowly scoped support edits that raise coverage
  without changing product behavior.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after both gates pass.
- Commit successful work after gates pass. Use conventional commit wording.
- After each successful commit, run `bin/integrate_stream_commit`.
- Treat the helper retry loop as part of the required success path.
- Do not commit blocked or failing work.
- Handoff must include final gate results and commit hash.
- Local model context is small: read only the docs and code needed for the
  chosen coverage task.

Progress source:

- The task ledger is canonical.
- Only choose from the `## Coverage Hardening` section.
- Ignore later sections while coverage-hardening tasks remain unchecked.

Project:

- Go CLI for local-first Jira Markdown workspace.
- Prefer stdlib; justify new dependencies.
- Coverage work should improve `bin/test` package totals or exercise missing
  branches, not rewrite features.

## Current Git Status

{{ commands.git_status }}

## Task Ledger

{{ commands.task_ledger }}

## Recent Commits

{{ commands.recent_commits }}

## Current Gate Output

{{ commands.gates }}

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
- gates (`bin/test && bin/lint`): <pass/fail>
- bin/integrate_stream_commit: <pass/fail>
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
