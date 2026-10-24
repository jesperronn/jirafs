# B102: Retry transient Jira failures (429, 503) with backoff.

## Dependencies

B101

## Owned paths

`internal/jira/**`

## Context

`SearchIssues` and the other request paths call `c.httpClient.Do(req)` once (e.g. `internal/jira/client.go:428-445`) and map any 4xx/5xx straight to an error via `mapHTTPErr`. A whole-project mirror refresh (B107 onward) makes hundreds of requests and will hit Jira Cloud rate limits (429, `Retry-After`) and brief 503s; today the first one aborts the run.

## Acceptance

- One shared `do(req)` helper in the client wraps `httpClient.Do` for all methods. It retries on 429 and 503 (and on connection-reset transport errors for idempotent GET/PUT only), up to 4 attempts.
- Wait time: the `Retry-After` header (seconds or HTTP-date) when present, capped at 60s; otherwise exponential backoff 1s, 2s, 4s with jitter. Sleeping respects context cancellation.
- Request bodies are replayable (`req.GetBody`) so POST search and PUT edits retry safely; `POST /issue` (create) and transitions are never retried.
- Sleep is injected (`WithSleep(func(ctx, d) error)` option) so tests do not wait.
- After the last attempt the original error is returned, and its message says how many attempts were made.

## Verification

- `bin/verify` passes.
- New tests in `internal/jira/client_test.go` using a stub transport: 429 with `Retry-After: 2` then 200 succeeds and the injected sleep received 2s; three 503s then 200 succeeds; four 503s fails with "after 4 attempts"; 429 on `CreateIssue` is not retried; context cancellation during sleep returns promptly.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
