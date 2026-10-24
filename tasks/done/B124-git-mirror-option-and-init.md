# B124: Opt-in git-backed mirror: `git: true` setting and `jirafs init --git`.

## Dependencies

B113, B115

## Owned paths

`internal/gitx/**` (new), `internal/cli/init.go`, `internal/config/**`, `internal/mirror/**`, `bin/test`, `README.md`

## Context

`runInit` (`internal/cli/init.go:55`) writes `mirror.yml` and the registry files from a `layoutFile` list (`init.go:116-123`). Settings load via `internal/config/settings.go`. Go has no git in-tree; shell out to the `git` binary (no go-git, per the few-dependencies rule). `bin/test` replaces `HOME` and prepends credential stubs to `PATH` (hermetic block), and `git` is still found on `PATH`.

## Acceptance

- `mirror.yml` (or the project's settings entry; choose and document one) accepts `git: true`; default false and everything else unchanged when off.
- `jirafs init --git` writes `git: true` and runs `git init` in the mirror dir if it is not already inside a repo (an existing repo, including a parent repo, is reused; say which). Writes a `.gitignore` containing `.jirafs/backup/`, `.jirafs/intents/`, and `.jirafs/tmp/`, and `.jirafs/base/` (base is derived data; B127 prefers the sync commit). Existing `.gitignore` is appended to, never overwritten.
- `internal/gitx` is a thin wrapper: `Available()`, `Run(dir, args...)`, `IsRepo`, `Toplevel`, `Status(paths)`, `Commit(paths, msg)`; no output parsing beyond porcelain v1 `-z`.
- Missing git: `jirafs: git is not installed or not on PATH; install it (brew install git / apt install git) or run: jirafs init` without `--git`. Missing `user.name`/`user.email` on commit: hint `git config --global user.name "Your Name"` and `git config --global user.email you@example.com`.
- Test helper `gitx.RequireGit(t)`: skips when git is missing, but fails when `JIRAFS_REQUIRE_GIT=1`; `bin/test` exports `JIRAFS_REQUIRE_GIT=1` so `bin/verify` always exercises the git tests. Tests use `t.TempDir()` repos, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, and `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env so they are hermetic.

## Verification

- `bin/verify` passes with git present; paste the output of a run showing the git tests executed (not skipped) via `go test -v ./internal/gitx/... | grep -i skip` (must print nothing).
- Tests: init --git in empty dir, in existing repo, in subdir of a parent repo, twice (idempotent), with PATH lacking git (hint text).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
