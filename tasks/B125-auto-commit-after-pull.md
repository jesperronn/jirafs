# B125: Git mode: auto-commit after `pull`.

## Dependencies

B113, B124

## Owned paths

`internal/cli/pull.go`, `internal/gitx/**`, `README.md`

## Context

After a pull, files jirafs wrote are known (the pull result lists written, merged, archived keys; B122 adds archive moves). A sync commit gives a history of what Jira changed and a merge base (B127). Users also edit these same files, so a blanket `git add -A` would sweep their work into a sync commit.

## Acceptance

- Only when `git: true` (B124) and not `--dry-run`. Stage exactly the paths pull wrote (explicit pathspecs, never `-A`), commit with subject `sync from <host> at <RFC3339 UTC timestamp>` and a body listing scope (or `KEY`) and `changed: KEY, KEY` plus `archived:` / `moved:` lines.
- Rule for user edits: before pull overwrites a file, if that file has uncommitted changes (`git status --porcelain -z`), jirafs never commits user work implicitly. Chosen rule: a file with uncommitted edits is merged per B120 and written, but is left out of the sync commit (reported: `left uncommitted: KEY (local edits)`); files without uncommitted edits are committed. Explain this rule in README.
- Nothing changed means no commit and no empty commit.
- Edge cases handled with a clear message and no commit (pull itself still succeeds): detached HEAD (commit anyway is NOT done; hint `git switch main`), rebase/merge in progress, index locked, repo toplevel outside the mirror dir (commit only paths inside the mirror dir), user commits interleaved between pulls (harmless; the sync commit only adds its paths).
- Commit failure never fails the pull; prints `pull ok, git commit failed: ...` with a copy-pasteable `git -C <dir> status`.

## Verification

- `bin/verify` passes.
- Temp-repo tests (B124 helper): clean pull commits with expected subject regex and body; second no-op pull makes no commit; user-edited file excluded and still on disk; detached HEAD skip; unrelated dirty file untouched and unstaged; user commit between pulls preserved.
- Paste `git log -2 --stat` from a test run.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
