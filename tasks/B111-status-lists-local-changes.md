# B111: `status` lists modified, draft and conflicted files.

## Dependencies

none

## Owned paths

`internal/cli/status.go`, `internal/mirror/**`, `internal/codec/**`, `README.md`

## Context

`BuildStatusSnapshot` (`internal/cli/status.go:72`) reports config, mirror files, scopes and a next step, but nothing about which local files have unsynced edits. README already lists "one obvious show me status command" as a rough edge. There is no stored content hash of the last synced state: the codec has hashing helpers (`grep -rn hash internal/codec/parse.go`) but sync does not persist the result of the last successful sync/export.

Rename note (phase 2): Hints naming `plan`/`sync`/`push FILE` become `diff`/`push` after B114/B115.

## Acceptance

- On export, refresh and successful sync, store the hash of the written issue body+frontmatter (excluding volatile fields) in the file's frontmatter as `synced_hash` (or in a sidecar `.jirafs/state.yml`; pick one and document it).
- `jirafs status` lists, grouped: `modified` (hash differs from `synced_hash`), `draft` (`state: draft`), `conflicted` (remote `updated` newer than the local `updated` and the file is modified; uses the mirror's stored remote timestamp, no network), each with the path and a copy-pasteable command: `jirafs plan KEY` or `jirafs push FILE`.
- Output is stable and sorted; a clean tree prints `no local changes`. `--porcelain` prints `STATE<TAB>KEY<TAB>PATH` lines for scripts.
- Status stays offline.

## Verification

- `bin/verify` passes.
- New tests in `internal/cli/status_test.go`: clean, one edited, one draft, one with remote ahead (conflicted); porcelain golden output.
- Paste `status` output from the test fixture showing all three groups.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
