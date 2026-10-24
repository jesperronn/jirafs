# B104: `--help` never needs settings; help on stdout, misuse on stderr.

## Dependencies

B103

## Owned paths

`internal/cli/**`, `cmd/jirafs/**`

## Context

`runExportIssue`, `runArchiveIssue`, `runMirrorRefresh` and `runMirrorArchiveSweep` only see `--help` after `config.Load()` succeeded (`export.go:42` loads settings before `runExportIssue`; `mirror.go:57` loads before `refresh`/`archive-sweep`; the per-subcommand check is at `mirror.go:78-83`; `archive.go:226-233`). So `jirafs export issue --help` with no settings prints "cannot load settings" instead of help.

All help is written to stderr (`printExportHelp` uses `exportStderr`, `export.go:189-205`; same in `mirror.go:533-584`, `archive.go:247-261`, `sync.go:326`), and `--help` returns 0 only sometimes. `setup` and `use` print their own differently styled help, and the `setup` help text still carries the internal tag "(B018d)" (`cmd/jirafs/main.go:191, 211`; comment at `297`).

Rename note (phase 2): `export issue` and `mirror refresh` disappear in B113 (into `pull`); if B113 lands first, apply the same help-first rule to `pull`, `diff` and `push` instead.

## Acceptance

- Subcommand help (`export issue`, `archive issue`, `archive list`, `mirror refresh`, `mirror archive-sweep`) is detected before settings load. Verified by running with empty `HOME`.
- Help requested explicitly (`--help`, `-h`, `help`) is written to stdout and exits 0. Usage errors (unknown subcommand, missing key, bad flag) print one error line plus `Use --help for usage.` to stderr and exit 2 (today several return 1: `export.go:53`, `mirror.go:72`).
- `setup` and `use` use the same help printer style as the other commands (shared `printUsage(w, ...)` helper, colour via the existing `color` package). Remove "(B018d)" from the flag description and comments.

## Verification

- `bin/verify` passes.
- New table test over every command and subcommand: `--help` with empty `HOME` gives exit 0, non-empty stdout, empty stderr; an unknown flag gives exit 2, empty stdout, non-empty stderr.
- `grep -rn "B0[0-9][0-9]" cmd/jirafs/main.go` shows no task ids in user-visible strings. Paste output.
- Paste: `HOME=$(mktemp -d) ./jirafs mirror refresh --help >/dev/null; echo $?` (must print 0 and nothing before it).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
