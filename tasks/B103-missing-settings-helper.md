# B103: One copy-pasteable first-run message when settings are missing.

## Dependencies

none

## Owned paths

`internal/cli/**`, `cmd/jirafs/**`, `README.md`

## Context

Every command prints its own `jirafs <cmd>: cannot load settings: %v` and exits 1 when `config.Load()` fails: `sync.go:63`, `export.go:44`, `plan.go:43`, `mirror.go:59`, `board.go:41`, `status.go:218`, `doctor.go:189`, `registry.go:79, 162`, `archive.go:228, 235`, `use.go:93, 121, 156`, `cmd/jirafs/main.go:106, 134, 162` and the unhandled `new.go:200` and `push.go:93`. On a first run with empty `HOME` the user gets a raw error and no next step, which also violates the AGENTS.md rule that hints are copy-pasteable commands.

Rename note (phase 2): `setup` becomes `init` in B116 and `sync`/`export`/`plan`/`mirror` are renamed in B113-B115; the helper's hint text must use the then-current command name.

## Acceptance

- New helper `loadSettingsOrHint(stderr io.Writer, cmd string) (*config.Settings, bool)` in `internal/cli` (for example `settings_load.go`). When the error is the missing-file / missing-field kind (`config.IsSettingError(err, config.ErrMissingField)`, as `main.go:271` already does), print:

  ```
  jirafs <cmd>: no settings found (~/.jirafs/settings.toml)
  next step: jirafs setup --project <name> --key <KEY> --instance <name> --base-url https://<site>.atlassian.net --credential-ref env://JIRAFS_API_TOKEN --set-current
  ```

  Other errors (parse errors) keep the current message plus the path of the file.
- Replace every `config.Load()` call listed above with the helper (including `main.go`'s `use`; `setup` and `init` keep tolerating a missing file).
- README "If something fails" section shows the same command.

## Verification

- `bin/verify` passes.
- New table test in `internal/cli/settings_load_test.go` runs every command entry point with `HOME` set to an empty `t.TempDir()` and asserts the output contains `jirafs setup --project`, `--credential-ref env://` and `--set-current`, and exit code 1.
- Paste output of: `bin/build && HOME=$(mktemp -d) ./jirafs sync PLAT-1` (adjust to the built binary path).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
