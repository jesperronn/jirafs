# B117: Generate README command surface and `docs/cli.md` from help text; fail on drift.

## Dependencies

B104, B113, B114, B115, B116 (run last so the command set is final)

## Owned paths

`internal/cli/**`, `cmd/jirafs/**`, `bin/**`, `README.md`, `docs/cli.md`

## Context

`README.md` "Command Surface" (lists setup, use, mirror refresh, export issue, plan, sync) and `docs/cli.md` are hand-written, and the README itself notes that its docs lag the code ("Current Status", rough edges). Help text lives in per-command printers (`printExportHelp` `export.go:189`, `printMirrorHelp` `mirror.go:534`, `printArchiveHelp` `archive.go:247`, `printStatusHelp` `status.go:279`, ...), which makes it the natural single source.

## Acceptance

- Command help is defined once per command as data (name, summary, usage, flags) in a registry in `internal/cli`, used by both the `--help` printers (B104 style) and a generator.
- `bin/gendocs` (or `jirafs docs --markdown`, hidden from help) renders the command table to stdout. `README.md` and `docs/cli.md` contain marked regions (`<!-- BEGIN GENERATED: commands -->` ... `<!-- END GENERATED -->`) filled by the generator; the rest stays hand-written.
- `bin/gendocs --check` exits 1 with a diff and the fix command `bin/gendocs` when files are stale; `bin/verify` runs it.
- A Go test (`internal/cli/docs_test.go`) regenerates in memory and compares to the files, failing when they drift; it also asserts every command in the dispatcher (`cmd/jirafs/main.go` switch) has a registry entry.

## Verification

- `bin/verify` passes.
- Demonstrate drift detection: edit one flag description in a help definition without regenerating and paste the failing `bin/verify` / `go test ./internal/cli -run Docs` output, then regenerate and paste the passing run (do not commit the intermediate state).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
