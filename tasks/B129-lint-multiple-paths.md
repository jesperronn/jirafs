# B129: `bin/lint` accepts any number of paths.

## Dependencies

None.

## Owned paths

`bin/lint`, `bin/verify` (only if needed), `tests/` shell tests for bin scripts

## Context

`bin/lint` takes a single optional `PATH` and errors with "only one PATH argument is allowed". The global working rules require: any number of file/directory paths, `--autofix`, whole project when no args. Agents had to run it once per path all session. `bin/lint README.md` also feeds markdown to the Go toolchain (`README.md:1:1: illegal character U+0023 '#'`) while still printing ok.

## Acceptance

- `bin/lint a b c` lints each path; exit non-zero if any fails; one summary line.
- Non-Go files passed explicitly are routed to the relevant non-Go check (or skipped with a clear note), never to gofmt/go vet; no spurious parse warnings.
- `--autofix` works with multiple paths. No args still lints the whole repo exactly as today.
- `--help` documents the new usage and prints to stdout.

## Verification

- `bin/verify` passes.
- Shell test (same style as existing bin tests) covering: two Go dirs, one bad file among several (non-zero exit), README.md (no Go parse warning), `--autofix` with two paths.
- Paste output of `bin/lint internal/cli internal/jira README.md`.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
