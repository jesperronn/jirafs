# B106: Delete stray tracked `internal/cli/PROJ-42.md` and guard against test output in the tree.

## Dependencies

none

## Owned paths

`internal/cli/**`

## Context

`internal/cli/PROJ-42.md` is tracked (`git ls-files internal/cli/PROJ-42.md`; added by commit 4041e73 "add PROJ-42.md (untracked file)"). It looks like output of a test that wrote into the package directory (the current working directory of `go test`) rather than a temp dir. `smoke_test.go:20-115` uses `t.TempDir()` and `PROJ-42` as its key, but other CLI tests may resolve a relative local dir.

## Acceptance

- `git rm internal/cli/PROJ-42.md`.
- Find which test regenerates it: run `go test ./internal/cli/...` then `git status --short internal/cli`; fix the offending test(s) to use `t.TempDir()` (or set the cwd/local dir to it).
- Add a guard to `internal/cli/main_hermetic_test.go`: after `m.Run()`, fail the package if any new file exists under the package directory compared to a snapshot taken before.

## Verification

- `bin/verify` passes.
- Paste output of `git ls-files internal/cli | grep -c '\.md$'` (expect 0) and of `bin/test && git status --short` (expect no new untracked files under `internal/`).
- The new guard is demonstrated: temporarily writing a file from a test makes `go test ./internal/cli` fail (describe the check in the commit message; do not commit the temporary change).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
