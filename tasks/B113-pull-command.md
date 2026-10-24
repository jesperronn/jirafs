# B113: `jirafs pull [scope|KEY]` replaces `export issue` and `mirror refresh`.

## Dependencies

B105, B107, B109, B110

## Owned paths

`cmd/jirafs/**`, `internal/cli/**`, `README.md`, `docs/cli.md`

## Context

Fetching from Jira is spread over `export issue KEY` (`internal/cli/export.go:36-53`, dispatched at `cmd/jirafs/main.go:62`) and `mirror refresh <scope>` plus the shorthands `mirror my-issues|current-sprint|backlog|next-sprint` (`internal/cli/mirror.go:66-70`, dispatch at `main.go:60`). There are no users yet, so no compatibility layer is wanted.

## Acceptance

- New `jirafs pull [scope|KEY]`. An argument matching an issue key pattern (`[A-Z][A-Z0-9]+-\d+`) pulls that single issue (old `export issue` behaviour, including `--stdout`); anything else is a scope name from `mirror.yml` (B107), refreshed incrementally (B109) with failure isolation (B110, `--full` kept). No argument pulls every configured scope. A name that is both is an error telling the user to rename the scope.
- Delete `export` (`export.go` command wiring, `main.go:62`) and `mirror refresh` plus the shorthands. `mirror archive-sweep` is not part of pull; move it to `jirafs archive sweep` in the same task so `mirror` can be deleted entirely. No aliases, no deprecation messages; old names fall to "unknown command".
- `pull` supports `--dry-run` (prints what would be fetched, and the real command) and `--project`.
- Update every error hint and help text that mentions `export issue`, `mirror refresh`, `mirror <scope>` (grep for them; the B110 retry hint becomes `jirafs pull KEY`).

## Verification

- `bin/verify` passes.
- Ported tests: `export_test.go`, `mirror_test.go`, `mirror_extra_test.go` rewritten against `pull`; new test that `jirafs export` and `jirafs mirror` exit 2 with unknown command.
- Paste: `grep -rn "export issue\|mirror refresh" internal cmd README.md docs` (must be empty).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
