# Ralph Stream WS2: Settings, Context, Registry, References

Purpose: one-task-at-a-time ledger for `ralphs/pi-ws2-settings-references`.

Rules:

- Pick the first unchecked task whose deps are checked.
- Stay in `internal/config/**`, `internal/context/**`,
  `internal/registry/**`, and `internal/references/**`.
- Add tests with new code.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after gates pass.
- Commit implementation, tests, and this ledger checkbox in one conventional
  commit.

Task format: `ID | deps | acceptance`

Completed dependency IDs archived in [Ralph Task Archive](ralph-task-archive.md):
`B001`, `B010`, `B011`, `B012`, `B013`, `B014`.

- [x] B015a | B013 | Read remembered current project when no explicit project or cwd match exists.
- [x] B015b | B015a | Write remembered current project after successful explicit selection.
- [x] B016a | B013 | Non-interactive unresolved context returns a structured no-project error.
- [x] B016b | B016a | Interactive unresolved context selects from known projects.
- [x] B040a | B011,B023b | Load user and project registry files with structured file errors.
- [ ] B040b | B040a | Load status, sprint, and fix-version registry files.
- [ ] B041a | B040a | Resolve user and project typed refs to Jira ids.
- [ ] B041b | B041a,B040b | Resolve status, sprint, and fix-version typed refs to Jira ids.
- [ ] B042a | B041a | Missing refs include type and lookup value.
- [ ] B042b | B042a | Ambiguous refs include candidate context.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
