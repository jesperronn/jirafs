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

Completed dependency IDs archived in [Ralph Task Archive](ralph-task-archive.md):
`B001`, `B020a`, `B020b`, `B020c`, `B021a`, `B021b`, `B021c`.

- [x] B021a | B020c | Define issue identity and machine-owned frontmatter fields.
- [x] B021b | B021a | Define editable issue fields and fixed section names.
- [x] B021c | B021b | Validate required issue fields and unknown sections.
- [ ] B022a | B021a | Define remote version, content hash, and sync timestamp metadata.
- [ ] B022b | B022a | Validate syncable, unsynced, archived, and draft states.
- [ ] B023a | B020c | Define user and project registry models.
- [ ] B023b | B023a | Define status, sprint, and fix-version registry models.
- [ ] B024a | B021b | Define typed plan operation model for one editable field.
- [ ] B024b | B024a | Define conflict model without Jira transport dependencies.
- [ ] B030a | B021c | Parse valid synced issue frontmatter into schema model.
- [ ] B030b | B030a | Parse valid draft issue frontmatter into schema model.
- [ ] B030c | B030b | Return structured errors for invalid frontmatter.
- [ ] B031a | B030a | Parse description and acceptance sections.
- [ ] B031b | B031a | Reject unknown sections explicitly.
- [ ] B032a | B031a | Render frontmatter with stable field order.
- [ ] B032b | B032a | Render fixed sections with stable section order.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
