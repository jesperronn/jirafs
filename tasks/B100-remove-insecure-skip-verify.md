# B100: Stop disabling TLS certificate verification by default.

## Dependencies

none

## Owned paths

`internal/jira/**`, `internal/config/**`, `README.md`

## Context

`defaultTransport` in `internal/jira/client.go:291-298` builds an `http.Transport` with `TLSClientConfig: &tls.Config{InsecureSkipVerify: true}` (line 296, silenced with `//nolint:gosec`), and `NewJiraClient` uses it for every production client (`client.go:305`). Every credential (basic, API token, bearer) is therefore sent over connections that accept any certificate. The doc comment at `client.go:287-290` calls this "useful for self-signed certs in dev environments".

## Acceptance

Production clients verify certificates using the system roots. Remove `InsecureSkipVerify` and the `nolint:gosec`. The loopback-guard branch (`http.DefaultTransport` replaced by the test harness, `client.go:291-293`) keeps working.

Only if self-signed Jira instances are a real need: add an explicit per-instance opt-in `insecure_skip_tls_verify` (default false) in `settings.toml`, wired through a `WithInsecureSkipTLSVerify` client option; when on, every command that creates a client prints a one-line warning to stderr. Otherwise do not add the flag. State the decision in the commit message.

README gets a short note on how to trust a private CA (system trust store) instead.

## Verification

- `bin/verify` passes.
- New test in `internal/jira/client_test.go`: an `httptest.NewTLSServer` with an untrusted cert is rejected by a client built with the production transport option (error mentions `x509`); with the opt-in (if added) it succeeds.
- `grep -rn InsecureSkipVerify internal cmd` shows no non-test hit (or only the opt-in path). Paste the output.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
