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
- [x] B040b | B040a | Load status, sprint, and fix-version registry files.
- [x] B041a | B040a | Resolve user and project typed refs to Jira ids.
- [x] B041b | B041a,B040b | Resolve status, sprint, and fix-version typed refs to Jira ids.
- [x] B042a | B041a | Missing refs include type and lookup value.
- [x] B042b | B042a | Ambiguous refs include candidate context.
- [ ] B017a | B012 | Parse one credential ref string into `scheme` and `target`, and return `ErrInvalidCredentialRef` for malformed refs.
- [ ] B017b | B017a | Accept only `env://` and `file://` schemes in the first implementation and reject unsupported schemes with structured errors.
- [ ] B017c | B017b | Parse every instance `credential_refs` entry into an ordered typed slice without resolving provider values yet.
- [ ] B017d | B017c | Resolve `env://VAR_NAME` credentials into normalized auth fields.
- [ ] B017e | B017c | Resolve `file://path` credentials into normalized auth fields.
- [ ] B017f | B017d,B017e | Merge ordered credential sources with later-source override.
- [ ] B017g | B017f | Validate required resolved auth fields by instance `auth_type`.
- [ ] B017h | B017g | Expose resolved instance credentials through a path-local API for Jira callers.

Integration handoff after each commit:

- Run `bin/integrate_stream_commit`.
- Report stream, commit, rebase/test/lint/push results from the helper output.
