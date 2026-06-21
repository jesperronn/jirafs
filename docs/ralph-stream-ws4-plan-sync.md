# Ralph Stream WS4: Plan And Sync

Purpose: one-task-at-a-time ledger for `ralphs/pi-ws4-plan-sync`.

Rules:

- Pick the first unchecked task whose deps are checked.
- Stay in `internal/plan/**` and `internal/sync/**`.
- Add tests with new code.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after gates pass.
- Commit implementation, tests, and this ledger checkbox in one conventional
  commit.

Task format: `ID | deps | acceptance`

Completed dependency IDs archived in [Ralph Task Archive](ralph-task-archive.md):
`B001`, `B020a`, `B020b`, `B020c`, `B021a`, `B021b`, `B021c`, `B022a`,
`B022b`, `B023a`, `B023b`, `B024a`, `B024b`, `B040a`, `B040b`, `B041a`,
`B041b`, `B042a`, `B042b`, `B050a`, `B050b`, `B051a`, `B051b`, `B052a`,
`B052b`, `B062a`, `B062b`.

- [x] B060a | B024b,B052a | Unchanged local/remote summary and description produce empty plan.
- [x] B060b | B060a,B052b | Unchanged refs and metadata produce empty plan.
- [x] B061a | B060a | Summary and description changes become typed operations.
- [x] B061b | B061a | Labels, assignee, status, sprint, and fix-version changes become typed operations.
- [x] B062a | B060b | Stale remote version produces conflict, not operations.
- [x] B062b | B062a | Stale content hash produces conflict, not operations.
- [x] B063a | B061a | Sync applies a validated no-op plan without mutation.
- [x] B063b | B063a,B061b | Sync applies one validated field-change operation.
- [x] B064a | B063b | Archive paths and unresolved refs fail before mutation.
- [ ] B064b | B064a | Stale state and invalid transitions fail before mutation.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
