# B107: Configurable scopes in `mirror.yml` (project, arbitrary JQL); drop unbacked scopes.

## Dependencies

none

## Owned paths

`internal/jira/**`, `internal/mirror/**`, `internal/cli/mirror.go`, `README.md`

## Context

`mirror.yml` already models scopes as `Scope{Name, Type, Target}` with types `project`, `component`, `jql` (`internal/mirror/mirror.go:74-111`), but `RefreshScope` only passes the scope *name* to the client (`internal/mirror/refresh_scope.go:22`: `c.SearchIssues(ctx, scope.Name)`), and `SearchIssues` hardcodes two names (`internal/jira/client.go:393-404`): `my-issues` -> `assignee = currentUser()`, `current-sprint` -> `sprint in openSprints()`; anything else is `not_found`. `Type`/`Target` are never read.

`backlog` and `next-sprint` are advertised but unbacked: accepted at `internal/cli/mirror.go:69` and documented at `mirror.go:541-551, 572-578`, yet they fail with not_found at the client.

Rename note (phase 2): `mirror refresh <scope>` becomes `pull <scope>` in B113; keep the scope logic in `internal/mirror` so the rename is wiring only.

## Acceptance

- Change the client contract to `SearchIssues(ctx, jql string)`; callers build the JQL from the scope. New `mirror.Scope.JQL()`: `project` -> `project = "<Target>" ORDER BY key`; `jql` -> `Target` verbatim; `component` -> `project = ... AND component = ...` only if a project is known, else a validation error. Built-in names `my-issues` and `current-sprint` keep working when no matching scope is in `mirror.yml` (compat), expressed as JQL.
- `jirafs mirror refresh <scope-name>` reads the scope from `mirror.yml`; unknown names list the configured scope names plus a copy-pasteable example of adding one to `mirror.yml`.
- Remove `backlog` and `next-sprint` from the dispatch (`mirror.go:69`) and all help text, or implement them as documented JQL scopes in the default `mirror.yml`; pick one and say which in the commit message. Do not leave advertised scopes that fail.
- README shows a `mirror.yml` with a project scope (`type: project, target: PLAT`) and a JQL scope.

## Verification

- `bin/verify` passes.
- New tests: `internal/mirror/refresh_scope_test.go` asserts the JQL sent to the fake for each scope type; `internal/cli/mirror_test.go` asserts an unknown scope lists the configured names; a test asserts `grep`-style that help text lists only supported scopes.
- Paste: `grep -rn "next-sprint\|backlog" internal cmd README.md` output and justify every remaining hit.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
