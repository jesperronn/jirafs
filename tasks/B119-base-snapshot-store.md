# B119: Store the last synced snapshot of each issue as merge base.

## Dependencies

B111, B113

## Owned paths

`internal/mirror/**`, `internal/codec/**`, `internal/cli/pull.go`, `internal/cli/push.go`, `README.md`

## Context

Nothing records what the remote looked like at the last successful pull or push. B111 stores only a `synced_hash` (a digest, not content), which can say "modified" but cannot support per-field or three-way comparison (B118, B120). `RemoteMetadata` (`internal/schema/remote_metadata.go:22-23`) holds only `RemoteVersion` and `ContentHash`. Writes of issue files happen in `push.go:159-175` (create) and `sync.go:210` (update, becomes `push` in B115), and in the export path (becomes `pull`, B113).

## Acceptance

- After every successful pull/push write of an issue, store its rendered remote-side content (the same markdown `schema.RenderIssue` produces, volatile fields excluded) at `.jirafs/base/KEY.md` under the mirror dir. Write atomically (temp file plus rename). Document the location in README.
- Helper `LoadBase(mirrorDir, key) (*schema.Issue, bool)`; missing or unparseable base returns `false`, never an error that blocks pull.
- `.jirafs/` is already treated as jirafs-internal state: `jirafs init` and docs state it is not to be edited by hand.
- Archive/move (B122) moves the base with the file.
- When git mode (B124) is on, the base file is still written; B127 prefers the git sync commit when present.

## Verification

- `bin/verify` passes.
- Tests: pull writes the base, push updates it, failed push leaves the old base, missing base returns false, corrupt base returns false.
- Paste `ls .jirafs/base` from the test after a pull.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
