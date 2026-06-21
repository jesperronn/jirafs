# Ralph Stream WS1: Schema And Codec

Purpose: one-task-at-a-time ledger for `ralphs/pi-ws1-schema-codec`.

Rules:

- Pick the first unchecked task whose deps are checked.
- Stay in `internal/schema/**` and `internal/codec/**`.
- Add tests with new code.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after gates pass.
- Commit implementation, tests, and this ledger checkbox in one conventional
  commit.

Task format: `ID | deps | acceptance`

- [ ] B020 | B001 | One typed-ref representation for users, projects, statuses, sprints, versions, epics, issues.
- [ ] B021 | B020 | Issue model covers required frontmatter and fixed sections from `docs/issue-format.md`.
- [ ] B022 | B021 | Sync metadata validates remote version, hash, timestamps, syncable state.
- [ ] B023 | B020 | Registry models match `docs/references.md`.
- [ ] B024 | B021 | Plan/operation models type editable changes and conflicts without Jira transport.
- [ ] B030 | B021 | Parse synced/draft frontmatter; invalid frontmatter gives structured errors.
- [ ] B031 | B030 | Parse known sections; unknown sections fail.
- [ ] B032 | B031 | Render issue docs with stable field and section order.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
