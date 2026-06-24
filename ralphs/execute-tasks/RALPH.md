---
agent: claude -p --dangerously-skip-permissions
commands:
  - name: pending-tasks
    run: find tasks -maxdepth 1 -type f -name "*.md" -not -name "README.md" | sort
  - name: git-log
    run: git log --oneline -5
---

# Prompt

You are an autonomous coding agent running in a loop. Each iteration
starts with a fresh context.

## Pending tasks

The following `.md` files in `tasks/` are pending (one task per file,
sorted by filename):

{{ commands.pending-tasks }}

## Recent commits

{{ commands.git-log }}

## What to do

1. If the pending tasks list above is **empty**, print exactly
   `no tasks remaining` and stop — do nothing else this iteration.
2. Otherwise, pick the **first** file from the pending tasks list
   (lowest filename when sorted alphabetically).
3. Read that task file in full. It describes one unit of work.
4. Implement the task completely. No placeholder code, no TODO
   comments, no partial implementations.
5. Run the verification gates: `bin/test` and `bin/lint`. Both must
   pass before committing. If either fails, fix the code and re-run
   until both pass — do not commit failing work.
6. Once both gates pass, commit the work with a descriptive message
   like `feat: add X` or `fix: resolve Y`. Reference the task
   filename in the commit body if it helps future readers.
7. Move the task file from `tasks/` to `tasks/done/` using `git mv`
   (create `tasks/done/` if it does not already exist). Include the
   move in the same commit as the implementation, or a follow-up
   commit — whichever keeps history cleaner.
8. After a successful commit, merge back to `main` with
   `git merge --ff-only` from the current branch. If the fast-forward
   merge fails, leave `main` untouched and report the conflict in the
   handoff.

## Rules

- **One task per iteration.** Do not attempt a second task even if
  the first was small.
- Always work on the first pending task — do not skip ahead.
- Both `bin/test` and `bin/lint` must pass before any commit.
- Never delete a task file — always move it to `tasks/done/` so the
  history is preserved.
- If a task is unclear or blocked, add a note to the task file
  explaining what is blocking it and leave it in `tasks/` for a
  human to resolve. Then print `blocked: <filename>` and stop the
  iteration without moving the file.

## Required handoff

Return this at the end of every iteration:

```text
Task:
- <filename and objective>

Validation:
- bin/test: <pass/fail>
- bin/lint: <pass/fail>

Files changed:
- <paths>

Commit:
- <hash and subject, or none if partial/blocked>

Merge to main:
- <fast-forward succeeded | failed: <reason> | skipped: blocked>

Status:
- <done|partial|blocked>

Risks:
- <open issues or none>
```
