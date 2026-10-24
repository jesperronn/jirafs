# B114: `jirafs diff [KEY...]` replaces `plan`.

## Dependencies

B105

## Owned paths

`cmd/jirafs/**`, `internal/cli/**`, `README.md`, `docs/cli.md`

## Context

`plan` (`internal/cli/plan.go:41`, dispatched at `cmd/jirafs/main.go:64`) shows the planned write-back for one key (`plan.go:96`) and, in a second path, for a set of local files (`plan.go:268`). Sync dry run (`sync.go:204-205`) prints the same plan, so two commands show the same thing under unrelated names.

## Acceptance

- Rename `plan` to `diff`: `jirafs diff [KEY...]` with no key diffs all modified files (uses the B111 `modified` set when present, else every local issue file). Output keeps the current plan format; add a one-line summary `N issue(s) changed`.
- Read-only and offline-safe except for fetching the remote issue; exit 0 whether or not there are differences (use `--exit-code` for 1-on-diff like git).
- Delete `plan` and its dispatch. No alias. Update hints that say `jirafs plan KEY` (`sync.go:262, 275` hint strings, README, B111 text) to `jirafs diff KEY`.

## Verification

- `bin/verify` passes.
- `plan_test.go` renamed to `diff_test.go` and extended: no-arg diff covers two modified files; `--exit-code` returns 1 with changes and 0 without; `jirafs plan` is an unknown command.
- Paste: `grep -rn "jirafs plan" internal cmd README.md docs` (must be empty).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
