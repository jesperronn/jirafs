# B126: Git mode: auto-commit after `push --apply`.

## Dependencies

B115, B124, B125

## Owned paths

`internal/cli/push.go`, `internal/gitx/**`, `README.md`

## Context

`push` writes to Jira, then rewrites local files (`push.go:165-175` for create, `--write-local` for updates, B105/B115). The local files only change after remote success, so a commit mirrors what Jira really accepted.

## Acceptance

- Only with `git: true`, `--apply`, and at least one remote success. Subject `push to <host> at <RFC3339 UTC timestamp>`; body one line per issue: `KEY: fields summary, labels; transition To Do -> Done; comment added; created NEW-KEY (from draft.md)`.
- Commit happens after the remote call succeeded and the local file was written. Partial failure (B115 per-issue isolation): commit only the succeeded issues' files, and append `not pushed: KEY (reason)` to the body and print `committed N of M; failed: KEY, KEY`.
- A created draft stages both the new `KEY.md` and the removal of the draft file, in one commit.
- Same safeguards as B125 (explicit pathspecs, detached HEAD, in-progress operations, commit failure never turns a successful remote write into a failing exit; it prints the recovery command).
- User's uncommitted edits to a pushed file: those edits were just pushed, so the file is committed in full; edits to files not pushed are untouched.

## Verification

- `bin/verify` passes.
- Temp-repo tests with the fake transport: single update, create-from-draft (rename), mixed run with one failure (partial commit text), dry run (no commit), git off (no commit).
- Paste the commit message of the mixed-run test.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
