# B118: Per-field conflict detection against a base, replacing the summary-only check.

## Dependencies

B115, B119

## Owned paths

`internal/sync/**`, `internal/plan/**`, `internal/schema/conflict.go`, `internal/cli/push.go`

## Context

`validatePlan` (`internal/sync/sync.go:76-100`) reports every version or hash mismatch as a `ConflictBothEdited` on `EditableFieldSummary`, whatever field actually changed, so the user cannot tell what collided. Both checks are skipped entirely when `RemoteVersion` or `ContentHash` is empty on either side (`sync.go:78-81`, `sync.go:92-95`), so a file without those values (draft-turned-synced, hand-made, older mirror) is never checked at all. `plan.BuildPlan` (`internal/plan/plan.go:20`) compares local to remote with no base, so it cannot tell "remote changed" from "local changed". Conflict types live in `internal/schema/conflict.go:9-27`.

Rename note: the update path lives in `push` after B115. The base snapshot comes from B119.

## Acceptance

- A pure function (in `internal/sync` or `internal/plan`) `DetectConflicts(base, local, remote schema.Issue) []schema.Conflict` that compares each editable field (summary, description, status, labels, fix versions, assignee, custom fields; whatever `schema.EditableField` lists) and reports a `ConflictBothEdited` with the real `Field`, `LocalValue`, `RemoteValue` only when local != base AND remote != base AND local != remote.
- A field changed only locally is a plain op (no conflict); changed only remotely is not a conflict and is not overwritten by push (the plan has no op for it).
- Missing base (no snapshot, B119): treat every field where local != remote as potentially conflicting, reported as conflict with a hint to run `jirafs pull KEY` first. Empty `RemoteVersion`/`ContentHash` never skips the check.
- `validatePlan` calls it; the two summary-only blocks are deleted.
- Push output names the field(s): `conflict: description changed locally and remotely` plus `jirafs diff KEY`.

## Verification

- `bin/verify` passes.
- Table tests in `internal/sync/sync_test.go`: only local, only remote, both same value (no conflict), both different (conflict on exactly that field), two fields (one conflict, one clean), empty version/hash, missing base.
- Paste the conflict message for a description-only collision.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
