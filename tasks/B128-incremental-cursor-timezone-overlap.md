# B128: Incremental pull must not miss issues across timezones.

## Dependencies

None.

## Owned paths

`internal/mirror/**`, `internal/cli/pull*.go`, `README.md`

## Context

`Scope.SearchJQL()` (`internal/mirror/mirror.go:~157`) appends `updated >= "<cursor - cursorOverlap>"`, with `cursorOverlap = time.Minute` (`mirror.go:125`) and the time formatted in the machine's local zone. Jira evaluates JQL dates in the Jira user's profile timezone, so when the two differ the query window starts too late and issues are silently skipped. Pull already skips unchanged issues by comparing `updated`, so re-fetching an overlap is cheap.

## Acceptance

- The cursor window can never start later than the true cursor, for any machine/Jira timezone pair (UTC−12..UTC+14). Pick and document the approach: e.g. overlap of 26h, or format in UTC and subtract the max offset. Prefer the simplest correct one.
- Already-unchanged issues inside the overlap are still reported as `unchanged`, not `updated` (no file writes, no git commit).
- README "incremental" section states the overlap and why.

## Verification

- `bin/verify` passes.
- Table test over machine zones (set `time.Local` in the test) proving the emitted JQL lower bound is <= cursor interpreted in any Jira zone in the range.
- Test that a second pull with overlap writes no files (mtime unchanged).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
