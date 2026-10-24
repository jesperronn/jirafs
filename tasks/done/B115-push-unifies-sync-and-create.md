# B115: `jirafs push [KEY|file...]` replaces `sync KEY` and `push draft.md`.

## Dependencies

B105, B111, B114

## Owned paths

`cmd/jirafs/**`, `internal/cli/**`, `README.md`, `docs/cli.md`

## Context

Two commands write to Jira: `sync KEY` (update, `internal/cli/sync.go:77-232`) and `push draft.md` (create, `internal/cli/push.go:93-159`, drafts only, refused otherwise). `push` with no arguments operates on every locally modified file (as listed by `status`, B111).

## Acceptance

- `jirafs push [KEY|file...]`: for each target, a draft file (`state: draft`) is created (old `push` path, rewritten to `KEY.md`); an existing issue is updated (old `sync` path, including transitions). A KEY resolves to its file in `local_dirs`.
- No argument: all `modified` and `draft` files (B111). Dry run by default; `--apply` writes (B105 semantics); `--write-local` is kept from B105 for updates. `--apply` with `--dry-run` exits 2.
- Per-issue results summary and failure isolation: `KEY  ok|skipped (conflict)|failed  reason`, exit 1 if any failed. Conflicted files are skipped and reported.
- Delete `sync` (`cmd/jirafs/main.go:56`) and the old `push` semantics. Update hints: `jirafs sync KEY` becomes `jirafs push KEY` (`push.go` refusal hint, `sync.go:262, 275`, README).

## Verification

- `bin/verify` passes.
- Merge `sync_test.go`, `sync_transition_test.go`, `sync_remote_status_test.go`, `push_test.go`, `smoke_test.go` into `push`-based tests; add: mixed run with one draft, one modified, one conflicted, one clean (dry run: zero writes; `--apply`: create + update + skipped); `jirafs sync` is an unknown command.
- Paste the summary output of the mixed `--apply` test and `grep -rn "jirafs sync" internal cmd README.md docs` (must be empty).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
