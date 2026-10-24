# B101: HTTP client timeout and real contexts instead of nil.

## Dependencies

none

## Owned paths

`internal/jira/**`, `internal/cli/**`

## Context

`NewJiraClient` creates `&http.Client{Transport: ...}` with no `Timeout` (`internal/jira/client.go:305`; `WithTransport` at `client.go:272` does the same). A stalled Jira connection hangs the CLI forever.

Callers pass a nil context and rely on every client method to substitute `context.Background()` (`if ctx == nil` at `client.go:315, 394, 507, 552, 593, 641, 701, 771`). Call sites: `internal/cli/sync.go:102` (`client.FetchIssue(nil, key)`), `sync.go:217` (`DoTransition(nil, ...)`), `export.go:115, 121`, `plan.go:96, 268`, `push.go:159`. `mirror.go:143` already uses `stdcontext.Background()`, so there is no cancellation anywhere.

Rename note (phase 2): `sync.go`/`export.go`/`plan.go` call sites move to `push.go`/`pull.go`/`diff.go` in B113-B115; apply the fix wherever the call lives when this lands.

## Acceptance

- The client has a default overall timeout (30s) via `http.Client.Timeout`, overridable with a `WithTimeout(d)` option; a timeout surfaces as a transport error whose message names the timeout and the URL host.
- Every CLI call site passes a real `context.Context` created once per command from `signal.NotifyContext(context.Background(), os.Interrupt)`, so Ctrl-C cancels in-flight requests cleanly.
- Remove the `if ctx == nil` fallbacks from client methods (the interface contract is "ctx must be non-nil"); `grep -rn "(nil, " internal/cli` shows no nil context left in non-test code.

## Verification

- `bin/verify` passes.
- New test in `internal/jira/client_test.go`: an `httptest` server that sleeps longer than `WithTimeout(50ms)` makes `FetchIssue` return a transport error within 1s.
- New test in `internal/cli/sync_test.go`: a cancelled context passed to the run helper returns exit 1 without a Jira write.
- Paste output of `grep -rn "FetchIssue(nil\|DoTransition(nil\|CreateIssue(nil" internal cmd` (must be empty).

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
