# B109: Incremental refresh with a per-scope last-sync cursor.

## Dependencies

B107, B108

## Owned paths

`internal/mirror/**`, `internal/cli/mirror.go`, `README.md`

## Context

`RefreshScope` re-fetches the full result set every run and only records new membership (`internal/mirror/refresh_scope.go:22-47`); `materializeScopeIssues` is called with the whole scope (`internal/cli/mirror.go:143`). `Mirror` (`internal/mirror/mirror.go:138-146`) stores scopes and members but no timestamps, and `schema.Issue` context carries an `Updated` string (`internal/schema/issue.go:153`, parsed at `internal/schema/parse.go:251`). For a large project each refresh therefore costs a full download and rewrites every file.

Rename note (phase 2): The command is `mirror refresh` today and `pull` after B113.

## Acceptance

- `Scope` gains `last_synced` (RFC3339 timestamp, written to `mirror.yml` only after a fully successful refresh of that scope).
- With a cursor present, the scope JQL is wrapped: `(<scope jql>) AND updated >= "yyyy/MM/dd HH:mm"` (Jira JQL format, cursor minus a 1-minute overlap to tolerate clock skew).
- Issues whose `updated` equals the one already stored in the local file's frontmatter are skipped (no rewrite, so mtime and local edits stay untouched); the summary prints `fetched N, updated M, unchanged K`.
- `--full` flag ignores the cursor and refetches everything; `--dry-run` never advances the cursor.
- A refresh that ends with failures (B110) does not advance the cursor.

## Verification

- `bin/verify` passes.
- New tests: first refresh stores `last_synced`; second refresh sends a JQL containing `updated >=` (assert on the fake request); unchanged issue file bytes and mtime are identical after refresh; `--full` omits the clause; failed refresh leaves `last_synced` unchanged.
- Paste two consecutive refresh outputs from the fake-backed test showing `updated 3` then `updated 0, unchanged 3`.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
