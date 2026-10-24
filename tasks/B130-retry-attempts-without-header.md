# B130: Report retry attempts without the internal response header.

## Dependencies

None.

## Owned paths

`internal/jira/client.go`, `internal/jira/*_test.go`

## Context

B102 makes `c.do` retry 429/503. To append "(after N attempts)" to the final error without touching callers, `do` sets an internal `X-Jirafs-Attempts` header on the last `*http.Response` (`attemptsHeader`, `internal/jira/client.go:301`) and `mapHTTPErr` reads it back. Smuggling state through a server response header is fragile (a real server could send it) and surprising. Also, a cancelled context during a retry wait surfaces as a string-wrapped error, so `errors.Is(err, context.Canceled)` fails.

## Acceptance

- No synthetic response headers. Attempt count travels via a return value or a small wrapper type local to the client; callers' observable error text is unchanged.
- A response header `X-Jirafs-Attempts` sent by the server has no effect.
- Cancelling during a retry wait yields an error for which `errors.Is(err, context.Canceled)` is true, while keeping the existing message.

## Verification

- `bin/verify` passes.
- Existing B102 retry tests still pass unchanged in their assertions on error text.
- New tests: server-sent `X-Jirafs-Attempts: 99` ignored; `errors.Is(..., context.Canceled)` on cancel-during-wait.
- `grep -rn "X-Jirafs-Attempts" internal` returns only the test that asserts it is ignored.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
