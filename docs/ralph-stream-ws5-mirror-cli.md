# Ralph Stream WS5: Mirror, CLI, Archive

Purpose: one-task-at-a-time ledger for `ralphs/pi-ws5-mirror-cli`.

Rules:

- Pick the first unchecked task whose deps are checked.
- Stay in `internal/mirror/**`, `internal/archive/**`, `internal/cli/**`, and
  `cmd/jirafs/**`.
- Add tests with new code.
- Final gates: `bin/test` and `bin/lint`.
- Mark `[x]` only after gates pass.
- Commit implementation, tests, and this ledger checkbox in one conventional
  commit.

Task format: `ID | deps | acceptance`

Completed dependency IDs archived in [Ralph Task Archive](ralph-task-archive.md):
`B001`, `B010`, `B011`, `B012`, `B013`, `B014`, `B015a`, `B015b`, `B016a`,
`B016b`, `B021a`, `B021b`, `B021c`, `B022a`, `B022b`, `B023a`, `B023b`,
`B030a`, `B030b`, `B030c`, `B040a`, `B040b`, `B041a`, `B041b`, `B042a`,
`B042b`, `B050a`, `B050b`, `B051a`, `B051b`, `B052a`, `B052b`, `B053a`,
`B053b`.

- [x] B070a | B011,B014 | Model explicit issue imports with explainable membership reason.
- [x] B070b | B070a | Model named scope membership alongside explicit imports.
- [x] B071a | B070b | Resolved out-of-scope issues become archive-eligible.
- [x] B071b | B071a | Pinned or unsynced issues remain live.
- [x] B080a | B015b,B016a | `jirafs use --project` updates remembered project.
- [x] B080b | B080a,B016b | `jirafs use` interactive and non-interactive selection behavior matches docs.
- [x] B081a | B056c,B070b | `jirafs mirror refresh` resolves project context and calls refresh service interface.
- [ ] B081b | B081a | `jirafs mirror refresh` reports deterministic changed issue keys.
- [x] B082a | B071a | `jirafs mirror archive-sweep` reports eligible actions without mutation.
- [ ] B082b | B082a | `jirafs mirror archive-sweep --apply` calls archive service interface.
- [ ] B083a | B056a | Wire `jirafs export issue` through the real service path.
- [ ] B083b | B061b | Wire `jirafs plan` through the real service path.
- [ ] B083c | B063b | Wire `jirafs sync` through the real service path.
- [ ] B083d | B083a,B083b,B083c | Add a disposable real-data smoke runner for export-edit-plan-sync-reexport.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
