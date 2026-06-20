---
agent: pi
commands:
  - name: git_status
    run: git status --short
  - name: task_ledger
    run: sed -n '1,260p' docs/ralph-loop-implementation-tasks.md
  - name: recent_commits
    run: git log --oneline -5
  - name: tests
    run: bin/test
  - name: lint
    run: bin/lint
---
# jirafs Pi Implementation Loop

You are implementing `jirafs` one small step at a time.

Read the live task ledger and pick the first unchecked task whose dependencies
are complete. Complete exactly one task in this iteration, update the task
ledger checkbox only after validation passes, provide a handoff, then stop.

## Hard Rules

- Run at most two delegate implementors at a time.
- Keep the task path-local. Do not edit outside the owned paths unless the task
  explicitly names that file or directory.
- Add or update tests for all new code in the same iteration.
- Run `bin/test` and `bin/lint` after the final diff.
- Do not mark a task complete unless both `bin/test` and `bin/lint` pass.
- Do not claim verification that was not run against the final diff.
- If validation fails, fix the task before starting anything else.
- If blocked, stop with a partial handoff and do not continue into another task.

## Project Context

- Product goal: local-first Jira workspace with structured Markdown issue files
  and safe sync back to Jira.
- Implementation language: Go.
- Dependency policy: prefer the Go standard library; justify every new external
  dependency.
- Important docs:
  - `docs/implementation-roadmap.md`
  - `docs/implementation-packets.md`
  - `docs/ralph-loop-implementation-tasks.md`
  - `docs/orchestration-model.md`
  - `docs/development-rules.md`
  - `docs/verification-policy.md`
  - `docs/code-style.md`

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

Return exactly this structure:

```text
Task:
- one-sentence objective

Scope:
- owned paths
- explicit out-of-scope paths

Acceptance:
- concrete behaviors now true

Validation:
- `bin/test`: pass or fail, with final outcome
- `bin/lint`: pass or fail, with final outcome
- any targeted tests or manual checks

Files changed:
- exact paths

Status:
- done, partial, or blocked

Next smallest step:
- one follow-on action for the next builder or orchestrator

Risks:
- open questions, contract pressure, or known gaps
```

Stop after that handoff. Do not start a second task in the same iteration.
