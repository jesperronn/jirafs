# B121: Conflicted issues show in `status`, block `push`, and are resolvable.

## Dependencies

B111, B115, B120

## Owned paths

`internal/cli/status.go`, `internal/cli/push.go`, `internal/cli/resolve.go` (new), `README.md`

## Context

B120 records conflicts in frontmatter. `BuildStatusSnapshot` (`internal/cli/status.go:72`) and B111's `conflicted` group currently derive conflicts from timestamps only. B115 already skips conflicted files in `push` and reports them; this task makes that skip depend on the recorded conflicts and adds the way out.

## Acceptance

- `status` lists a `conflicted` entry for every file with a non-empty `conflicts:` frontmatter, with fields and the commands `jirafs resolve KEY --keep local|remote [--field NAME]` and `jirafs diff KEY`. `--porcelain` emits `conflicted` lines.
- `push` refuses files with `conflicts:` (result `skipped (conflict)`, exit 1 in `--apply` if explicitly named), regardless of timestamps.
- `jirafs resolve KEY --keep local|remote [--field NAME]`: local keeps the local value and drops the conflict entry; remote takes the stored remote value after first saving local to `.jirafs/backup/` (B120). Removing the last conflict clears the `conflicts:` key. Dry run by default, `--apply` writes (B105 semantics).
- Editing the file by hand and deleting the `conflicts:` block is also accepted (documented in README).

## Verification

- `bin/verify` passes.
- Tests: status shows conflict, push skips it, resolve local, resolve remote with backup, per-field resolve, partial resolve keeps the rest.
- Paste `status` and `resolve` output.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
