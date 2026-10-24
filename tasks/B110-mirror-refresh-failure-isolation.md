# B110: Per-issue failure isolation in `mirror refresh`.

## Dependencies

none (cooperates with B109: failures hold the cursor back)

## Owned paths

`internal/cli/mirror.go`, `internal/mirror/**`

## Context

`materializeScopeIssues` is the single place that writes issue files for a scope (called at `internal/cli/mirror.go:143`); the error from it fails the whole refresh. One malformed issue (bad custom field, unwritable path, render error) therefore aborts a refresh of hundreds of issues and leaves a partially updated mirror with no list of what was skipped.

Rename note (phase 2): The command is `mirror refresh` today and `pull` after B113; the retry hint becomes `jirafs pull KEY` then.

## Acceptance

- Issue processing continues after a per-issue error; each failure records key plus reason.
- Final output lists failures (`key: reason`, one per line, stderr) and a summary `refreshed N, failed M`; exit code is 1 if any failed, 0 otherwise. Fatal errors (auth, search failure) still abort immediately.
- `mirror.yml` membership is saved for the issues that succeeded.
- The failure list ends with a copy-pasteable retry per key: `jirafs export issue KEY`.

## Verification

- `bin/verify` passes.
- New test in `internal/cli/mirror_test.go`: a fake with 3 issues where the middle one fails to render still writes the other two, exits 1, and stderr contains the failing key and `jirafs export issue <KEY>`.
- Paste that test's stdout/stderr.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
