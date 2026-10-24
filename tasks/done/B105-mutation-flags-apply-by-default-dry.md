# B105: Consistent mutation flags: Jira writes are dry-run unless `--apply`.

## Dependencies

B104

## Owned paths

`internal/cli/**`, `README.md`

## Context

`push` is dry-run by default with `--apply` (`push.go:64`), but `sync` writes to Jira by default and its `--apply` flag only means "also write the updated remote to the local file" (`sync.go:43`, `sync.go:226-231`; help text at `sync.go:326`). `--dry-run` exists on sync/push/export/new/archive/mirror (README "If something fails"), and `dryRunConflicts` (`internal/cli/dryrun.go:12-18`) already rejects `--dry-run` with `--apply`, but for sync that rejects two flags with different meanings. The same word therefore means "write to Jira" in one command and "write the local file" in another.

Rename note (phase 2): `sync` becomes `push` in B115. This task must land first: B115 relies on these flag semantics.

## Acceptance

- Rule for every command that writes to Jira (`sync`, `push`, and any other found by `grep -rn "DoTransition\|UpdateIssue\|CreateIssue" internal/cli`): no writes without `--apply`; running without `--apply` behaves as `--dry-run`.
- `sync`: the old local-file flag is renamed to `--write-local` (writes the refreshed remote to the local file after a successful apply; no effect in dry run). The old `--apply` spelling now means "perform the Jira writes".
- `--apply` together with `--dry-run` is rejected on all of them with the existing `dryRunConflicts` message, exit 2.
- Every dry run ends with `dryRunDone(...)` printing the exact real command including all flags the user passed (`jirafs sync PLAT-123 --apply --write-local`), project flag included.
- README: note under Current Workflow and the example block that `sync` now needs `--apply`, explicitly flagged as a behaviour change (breaking for scripts that relied on `jirafs sync KEY` writing).

## Verification

- `bin/verify` passes.
- Update `sync_test.go`, `sync_transition_test.go`, `sync_remote_status_test.go` and `smoke_test.go:106`: add a test that `sync KEY` without `--apply` performs zero fake-transport writes and prints `to run it for real: jirafs sync KEY --apply`; one that `--apply --dry-run` exits 2; one that `--write-local` is rejected without effect in dry run.
- Paste output of a dry run against the fake transport showing the printed real-run command.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
