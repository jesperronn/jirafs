# Ralph Stream WS3: Jira And Export

Purpose: one-task-at-a-time ledger for `ralphs/pi-ws3-jira-export`.

Rules:

- Pick the first unchecked task whose deps are checked.
- Stay in `internal/jira/**` and `internal/export/**`.
- Add tests with new code.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after gates pass.
- Commit implementation, tests, and this ledger checkbox in one conventional
  commit.

Task format: `ID | deps | acceptance`

Completed dependency IDs archived in [Ralph Task Archive](ralph-task-archive.md):
`B001`.

- [x] B050a | B021a | Define Jira client interface and request/response error type.
- [x] B050b | B050a | Add fake transport for JSON tests without network.
- [x] B051a | B050b | Fetch one issue payload by key into remote data.
- [x] B051b | B051a | Map Jira fetch errors to structured client errors.
- [x] B052a | B021b,B051a | Normalize summary, description, labels, and assignee into issue model.
- [x] B052b | B052a | Normalize linked issues as shallow typed refs.
- [x] B053a | B050b | Search fake `my-issues` scope with deterministic keys.
- [x] B053b | B053a | Search fake `current-sprint` scope with deterministic keys.
- [x] B054a | B017h,B050b | Build authenticated Jira requests from resolved credentials.
- [x] B054b | B054a | Add current-user lookup helper for scope resolution.
- [x] B055a | B054a,B053a | Implement real Jira search for `my-issues`.
- [x] B055b | B054b,B053b | Implement real Jira search for `current-sprint`.
- [x] B056a | B032b,B051a | Export one fetched issue through the canonical codec.
- [x] B056b | B056a,B040b | Refresh project-scoped registries needed by exported refs.
- [ ] B056c | B055b,B056b | Refresh one named mirror scope while keeping linked issues shallow.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
