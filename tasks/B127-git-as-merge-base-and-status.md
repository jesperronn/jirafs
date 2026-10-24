# B127: Git mode: use the last sync commit as merge base and report unpushed edits in `status`.

## Dependencies

B119, B120, B125, B126, B111

## Owned paths

`internal/cli/status.go`, `internal/cli/pull.go`, `internal/gitx/**`, `internal/merge/**`, `README.md`

## Context

B120 merges against the stored base (B119 `.jirafs/base/KEY.md`). With git mode the same content exists in history: the last commit whose subject starts with `sync from ` or `push to ` is the last known remote state of every file it touched. `status` today (`internal/cli/status.go:72`) has no notion of commits.

## Acceptance

- `gitx.LastSyncCommit(dir)` finds the newest commit (on the current branch, `git log --grep '^\(sync from\|push to\) ' -n 1`); `gitx.ShowFile(commit, path)` returns base content. Pull in git mode uses it as base for B120 when the file exists at that commit, else falls back to B119's base, else no-base behaviour.
- Edge cases stated in README and tested: user commits interleaved (base is still the last sync/push commit, user commits count as local edits), history rewritten or commit missing (fallback to stored base), file added after the sync commit (no base), shallow clone, detached HEAD (read-only use is fine).
- `status` in git mode adds `N local commits not pushed` (commits after the last sync/push commit touching mirror files) and `N files with uncommitted edits`, plus the commands `git -C <dir> log --oneline <sha>..HEAD` and `jirafs push`. Output is stable; `--porcelain` stays `STATE<TAB>KEY<TAB>PATH` lines only; unpushed commits appear as `unpushed-commit<TAB>SHA<TAB>SUBJECT` rows.
- Status stays offline.

## Verification

- `bin/verify` passes.
- Temp-repo tests: base from sync commit equals stored base; interleaved user commit; rewritten history fallback; status counts before and after a push commit.
- Paste `status` output with 2 local commits and 1 dirty file.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
