# B122: Handle deleted and moved issues on `pull` without deleting files.

## Dependencies

B113, B119

## Owned paths

`internal/cli/pull.go`, `internal/mirror/**`, `internal/archive/**`, `internal/schema/remote_metadata.go`, `README.md`

## Context

Deletes and moves are rare, so keep this lightweight. `FetchIssue` returns a `NewNotFoundError` for a missing issue (`internal/jira/errors.go:93`, `internal/jira/client.go:313-320`); today pull would just fail (B110 isolates it). A moved issue is returned under a new `key` (the fetched JSON `key` field, `internal/jira/client.go:26`) that differs from the requested one. The archive directory is `_archive` inside each local dir (`internal/cli/mirror.go:421-435`, `archive.Service` at `internal/archive/archive.go:35-72`).

## Acceptance

- Pull classifies a mirrored issue as `gone` (not found) or `moved` (response key differs). Other errors keep the B110 behaviour.
- Never delete a file. Gone: set frontmatter `remote_state: gone`, move the file into `_archive`. Moved: set `remote_state: moved`, `moved_to: NEW-KEY`, write the content under the new key's filename (and update mirror membership to the new key), and move the old file to `_archive`.
- One line per issue, copy-pasteable: `gone PLAT-9: moved to _archive/PLAT-9.md (restore: mv _archive/PLAT-9.md .)` and `moved PLAT-9 -> PLAT-12: jirafs pull PLAT-12`.
- Locally modified gone issues keep their local edits in the archived file and the line says so. `push` refuses archived/gone files with a hint.
- Dry run (`pull --dry-run`) prints the same lines prefixed `would`.

## Verification

- `bin/verify` passes.
- Fake-transport tests: 404 issue archived with `remote_state: gone`; moved issue renamed and archived under old key; modified file preserved; dry run changes nothing.
- Paste the printed lines for both cases.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
