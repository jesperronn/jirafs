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

- [ ] B050 | B021 | Jira client interface and fake transport allow JSON tests without network.
- [ ] B051 | B050 | Fetch one issue payload into normalized remote data.
- [ ] B052 | B021,B051 | Normalize one remote issue into canonical issue model; links stay shallow refs.
- [ ] B053 | B050 | Fake `my-issues` or `current-sprint` search returns deterministic issue keys.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
