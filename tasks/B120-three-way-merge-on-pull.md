# B120: Three-way merge on `pull` for locally modified issues.

## Dependencies

B113, B118, B119

## Owned paths

`internal/cli/pull.go`, `internal/merge/**` (new), `internal/schema/render.go`, `README.md`

## Context

Today a refresh overwrites the local file with the remote version (export path, `internal/cli/export.go:36-53`; scope refresh in `internal/mirror/refresh_scope.go:17`), which silently discards unsynced local edits. With a base (B119) and field comparison (B118) the pull can merge instead.

## Acceptance

- For each pulled issue whose local file differs from base: per field (B118 rules) take remote where only remote changed, keep local where only local changed, and mark the field conflicted where both changed differently. Unmodified files are overwritten as before.
- Long text (description): merge by line using a small in-repo diff3 (no new dependency); overlapping hunks are conflicted.
- A conflicted issue keeps the local value in the field, stores the remote value in frontmatter as `conflicts: [{field, remote, base}]`, and does not update the base for the conflicted fields (so the next pull reproduces them). Non-conflicted merged fields are written and the file stays "modified".
- `pull` prints one line per issue: `merged KEY` or `conflict KEY description` with the copy-pasteable `jirafs diff KEY`.
- No local edit is ever lost: local content before the merge is saved to `.jirafs/backup/KEY-<timestamp>.md` whenever a merge or conflict occurs.
- No base (B119): never overwrite a modified file; report it as conflicted instead.

## Verification

- `bin/verify` passes.
- Merge table tests: disjoint fields, same field one side, both same value, disjoint description hunks, overlapping hunks, no base, unmodified file.
- Paste the `pull` output for one merged and one conflicted issue and `diff` of the backup.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
