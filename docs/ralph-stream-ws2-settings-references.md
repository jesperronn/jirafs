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

- [x] B010 | B001 | Settings errors expose stable codes and messages.
- [x] B011 | B010 | Parse `~/.jirafs/settings.toml`; load instances/projects; expand paths.
- [x] B012 | B011 | Reject duplicate keys, missing instances, bad project refs, bad mirror dirs.
- [x] B013 | B011 | Explicit `--project` beats every other source.
- [x] B014 | B013 | Cwd mapping uses most-specific match; ambiguity fails clearly.
- [ ] B015 | B013 | Remembered current project is read/written and lower precedence.
- [ ] B016 | B013 | Non-interactive unresolved context fails without prompting.
- [ ] B040 | B011,B023 | Load each registry family with structured file errors.
- [ ] B041 | B040 | Resolve all typed refs to Jira ids through registries.
- [ ] B042 | B041 | Missing/ambiguous refs include type, lookup value, candidates when available.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
