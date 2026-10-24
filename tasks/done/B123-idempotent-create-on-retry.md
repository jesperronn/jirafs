# B123: `push` creating an issue must not duplicate it on retry.

## Dependencies

B115

## Owned paths

`internal/cli/push.go`, `internal/jira/**`, `internal/schema/remote_metadata.go`, `README.md`

## Context

The create path calls `client.CreateIssue` (`internal/cli/push.go:159`) and only afterwards writes `KEY.md` and removes the draft (`push.go:165-175`). If the response is lost (timeout, killed process) the issue exists in Jira but the draft is still `state: draft`; running push again creates a second issue. Comment writes do not exist in the codebase yet (`grep -rn AddComment internal` is empty); when a comment-add feature is added it must follow the same intent-then-verify rule described here.

## Acceptance

- Before the call, write an intent record `.jirafs/intents/<draft-basename>.yml` (random token, project, summary, timestamp) and add the token to the created issue as a label `jirafs-<token>` (or a hidden property if the client supports it; pick one and document it).
- On retry of a draft with an intent record: search Jira (`labels = "jirafs-<token>"`, using the existing search client) first; if found, adopt that key (write `KEY.md`, remove draft and intent, remove the label best-effort) instead of creating; if not found, reuse the token and create.
- Intent record is deleted after the local file is written. A stale intent (older than 7 days) is reported by `doctor`.
- Output on adoption: `found KEY created by an earlier attempt; not creating a duplicate`.

## Verification

- `bin/verify` passes.
- Fake-transport tests: create succeeds but the response is dropped (fake error after registering) then retry adopts the same key and `created == 1`; plain retry after a failed create creates once; intent file lifecycle.
- Paste the retry output.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
