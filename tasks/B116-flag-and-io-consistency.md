# B116: Consistent flag names, `init`/`setup` decision, stdout/stderr discipline.

## Dependencies

B103, B104

## Owned paths

`cmd/jirafs/**`, `internal/cli/**`, `README.md`, `docs/cli.md`

## Context

- `init` uses `--jira-url` (`internal/cli/init.go:65, 75-76, 252`) while `setup` uses `--base-url` (`cmd/jirafs/main.go:208, 242`).
- `setup --project` is the settings project *name* and `--key` the Jira key (`main.go:188-211`), whereas in `sync`/`plan`/`export` `--project` selects an existing configured project (`sync.go:70`). `init` and `setup` both create configuration and overlap.
- `status` (`internal/cli/status.go:206`) and `doctor` (`doctor.go:187`) take no `--project`/`--cwd`, unlike the other commands.
- `use` is implemented twice: `cmd/jirafs/main.go:104-162` and `internal/cli/use.go:91-156`; both iterate `s.Projects` as a map (`main.go:141, 149`, `use.go:134, 144`), so listing order is random.
- Data and hints are mixed on stderr: setup progress goes to stderr (`main.go:255-258`) and all help to stderr (B104 fixes help only).

## Acceptance

- `--base-url` everywhere; remove `--jira-url` outright.
- Decide `init` vs `setup`: merge into one command named `init` (flags of `setup`, plus what `init` does today beyond settings: mirror dir and default `mirror.yml`), delete `setup`. Update README, hints (B103 hint becomes `jirafs init --project ...`).
- `--project` means exactly one thing in every command: the name of a configured project from settings. In `init` the project name is the first positional argument and the Jira key stays `--key`. Document in `docs/cli.md`.
- `status` and `doctor` accept `--project` and `--cwd` through the shared flag helper in `flagparse.go`.
- One `use` implementation (delete the duplicate in `main.go`), project listing sorted by name.
- Rule applied everywhere: results/data (listings, plans, exported content, paths) go to stdout; progress, warnings, hints and errors go to stderr. Audit with `grep -n "Fprint.*Stderr" internal/cli cmd` and fix commands that print their primary result on stderr.

## Verification

- `bin/verify` passes.
- New tests: `init --jira-url` errors as unknown flag; `use` with 3 projects lists them sorted (run 20 times); `status --project X --cwd DIR` works; a table test that each command's primary output lands on stdout (`bytes.Buffer` split) for a happy-path run.
- Paste: `grep -rn "jira-url\|jirafs setup" internal cmd README.md docs` (must be empty).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
