# Ralph Loop Tasks

Purpose: small implementation steps for `ralphs/pi-implementation`.

Rules:

- Pick the first unchecked task whose deps are done.
- Do exactly one task, test it, commit it, hand off, then stop.
- Stay in owned paths. Add tests with new code.
- Required final gates: `bin/test` and `bin/lint`.
- Only mark `[x]` after gates pass and the task commit exists.
- Commit done work with conventional commit wording. Do not commit blocked work.
- The checked task ledger is the only progress pointer. Each iteration starts
  at the first unchecked task whose deps are checked.

Task format:
`ID | deps | owned paths | acceptance`

Completed dependency IDs archived in [Ralph Task Archive](ralph-task-archive.md):
`B001`, `B002`, `B003`, `B010`, `B011`, `B012`, `B013`, `B014`, `B020a`,
`B020b`, `B020c`, `B005a`, `B005e`, `B005f`, `B021b`, `B022b`.

## Foundation

- [ ] B004a | B001 | `bin/**`, `tests/**`, `tools/**` | Add bash test runner and one passing shell-test fixture wired into `bin/test`.
- [ ] B004b | B004a | `bin/test`, `tests/**` | Bash-test `bin/test` argument handling: default behavior and invalid args.
- [ ] B004c | B004a | `bin/lint`, `tests/**` | Bash-test `bin/lint` argument handling: default behavior and invalid args.
- [ ] B004d | B004a | `bin/integrate_stream_commit`, `tests/**` | Bash-test clean-worktree guard and default `main` target behavior.
- [ ] B004e | B004d | `bin/integrate_stream_commit`, `tests/**` | Bash-test retry/backoff after non-fast-forward push failure.
- [ ] B004f | B004d | `bin/integrate_stream_commit`, `tests/**` | Bash-test rebase, test, and lint failure paths stop before push.

## Coverage Hardening

- [x] B005a | B080b | `cmd/jirafs/**`, `tests/**` | Add `main` command tests for `mirror` routing and unknown subcommand/help edge cases to raise binary entrypoint coverage.
- [x] B005b | B081b | `internal/cli/**`, `tests/**` | Add focused `mirror` CLI tests for refresh/archive argument errors, project-resolution failures, and persistence edge cases.
- [x] B005c | B054a,B055b | `internal/jira/**`, `tests/**` | Add Jira client tests for auth/header construction and search/fetch error branches that still miss coverage.
- [x] B005d | B033b | `internal/schema/**`, `tests/**` | Add schema parse/render round-trip tests for zero-value, partial-metadata, and invalid-state edge cases.
- [x] B005e | B064b | `internal/sync/**`, `tests/**` | Add sync validation tests for remaining no-op, mismatch, and conflict formatting branches not yet covered.
- [x] B005f | B005a,B005b,B005c,B005d,B005e | `docs/**`, `tests/**` | Raise repo-wide `bin/test` coverage back to at least 90.0% and record the package-level additions in the task ledger handoff.

## Settings And Context

- [x] B015a | B013 | `internal/context/**`, `tests/context/**` | Read remembered current project when no explicit project or cwd match exists.
- [x] B015b | B015a | `internal/context/**`, `tests/context/**` | Write remembered current project after successful explicit selection.
- [x] B016a | B013 | `internal/context/**`, `tests/context/**` | Non-interactive unresolved context returns a structured no-project error.
- [x] B016b | B016a | `internal/context/**`, `tests/context/**` | Interactive unresolved context selects from known projects.
- [ ] B018a | B017h | `cmd/jirafs/**`, `internal/config/**`, `tests/**` | Add a setup helper that records Jira base URL, mirror directory, and auth source for one named project.
- [ ] B018b | B018a | `cmd/jirafs/**`, `internal/config/**`, `internal/context/**`, `tests/**` | Setup helper creates or validates the target mirror directory and writes a working settings file skeleton.
- [ ] B018c | B018b | `cmd/jirafs/**`, `internal/config/**`, `tests/**` | Setup helper supports API token configuration through `env://` or `file://` credential refs with clear operator guidance.
- [ ] B018d | B018c,B080a | `cmd/jirafs/**`, `internal/context/**`, `tests/**` | Setup helper can set the remembered current project so the first export/refresh flow works without extra flags.

## Schema

- [x] B021a | B020c | `internal/schema/**`, `tests/schema/**` | Define issue identity and machine-owned frontmatter fields.
- [x] B021b | B021a | `internal/schema/**`, `tests/schema/**` | Define editable issue fields and fixed section names.
- [x] B021c | B021b | `internal/schema/**`, `tests/schema/**` | Validate required issue fields and unknown sections.
- [x] B022a | B021a | `internal/schema/**`, `tests/schema/**` | Define remote version, content hash, and sync timestamp metadata.
- [x] B022b | B022a | `internal/schema/**`, `tests/schema/**` | Validate syncable, unsynced, archived, and draft states.
- [ ] B023a | B020c | `internal/schema/**`, `tests/schema/**` | Define user and project registry models.
- [ ] B023b | B023a | `internal/schema/**`, `tests/schema/**` | Define status, sprint, and fix-version registry models.
- [ ] B024a | B021b | `internal/schema/**`, `tests/schema/**` | Define typed plan operation model for one editable field.
- [ ] B024b | B024a | `internal/schema/**`, `tests/schema/**` | Define conflict model without Jira transport dependencies.

## Codec

- [ ] B030a | B021c | `internal/codec/**`, `tests/codec/**` | Parse valid synced issue frontmatter into schema model.
- [ ] B030b | B030a | `internal/codec/**`, `tests/codec/**` | Parse valid draft issue frontmatter into schema model.
- [ ] B030c | B030b | `internal/codec/**`, `tests/codec/**` | Return structured errors for invalid frontmatter.
- [x] B031a | B030a | `internal/codec/**`, `tests/codec/**` | Split issue body into ordered `##` section blocks after frontmatter.
- [x] B031b | B031a | `internal/codec/**`, `tests/codec/**` | Populate `Issue.Sections` for `Description` and `Acceptance Criteria`, including empty sections.
- [x] B031c | B031b | `internal/codec/**`, `tests/codec/**` | Reject unknown section headings explicitly.
- [x] B032a | B031b | `internal/codec/**`, `tests/codec/**` | Render frontmatter with stable field order.
- [x] B032b | B032a | `internal/codec/**`, `tests/codec/**` | Render fixed sections with stable section order.
- [x] B033a | B032b | `tests/codec/**` | Add synced issue round-trip golden fixture.
- [x] B033b | B033a | `tests/codec/**` | Add draft issue round-trip golden fixture.

## Registry And References

- [x] B040a | B011,B023b | `internal/registry/**`, `tests/registry/**` | Load user and project registry files with structured file errors.
- [x] B040b | B040a | `internal/registry/**`, `tests/registry/**` | Load status, sprint, and fix-version registry files.
- [x] B041a | B040a | `internal/references/**`, `tests/references/**` | Resolve user and project typed refs to Jira ids.
- [x] B041b | B041a,B040b | `internal/references/**`, `tests/references/**` | Resolve status, sprint, and fix-version typed refs to Jira ids.
- [x] B042a | B041a | `internal/references/**`, `tests/references/**` | Missing refs include type and lookup value.
- [x] B042b | B042a | `internal/references/**`, `tests/references/**` | Ambiguous refs include candidate context.

## Jira Read Path

- [x] B050a | B021a | `internal/jira/**`, `tests/jira/**` | Define Jira client interface and request/response error type.
- [x] B050b | B050a | `internal/jira/**`, `tests/jira/**` | Add fake transport for JSON tests without network.
- [x] B051a | B050b | `internal/jira/**`, `tests/jira/**` | Fetch one issue payload by key into remote data.
- [x] B051b | B051a | `internal/jira/**`, `tests/jira/**` | Map Jira fetch errors to structured client errors.
- [x] B052a | B021b,B051a | `internal/export/**`, `tests/export/**` | Normalize summary, description, labels, and assignee into issue model.
- [x] B052b | B052a | `internal/export/**`, `tests/export/**` | Normalize linked issues as shallow typed refs.
- [x] B053a | B050b | `internal/jira/**`, `tests/jira/**` | Search fake `my-issues` scope with deterministic keys.
- [x] B053b | B053a | `internal/jira/**`, `tests/jira/**` | Search fake `current-sprint` scope with deterministic keys.

## Planner And Sync

- [x] B060a | B024b,B052a | `internal/plan/**`, `tests/plan/**` | Unchanged local/remote summary and description produce empty plan.
- [x] B060b | B060a,B052b | `internal/plan/**`, `tests/plan/**` | Unchanged refs and metadata produce empty plan.
- [x] B061a | B060a | `internal/plan/**`, `tests/plan/**` | Summary and description changes become typed operations.
- [x] B061b | B061a | `internal/plan/**`, `tests/plan/**` | Labels, assignee, status, sprint, and fix-version changes become typed operations.
- [x] B062a | B060b | `internal/plan/**`, `tests/plan/**` | Stale remote version produces conflict, not operations.
- [x] B062b | B062a | `internal/plan/**`, `tests/plan/**` | Stale content hash produces conflict, not operations.
- [x] B063a | B061a | `internal/sync/**`, `tests/sync/**` | Sync applies a validated no-op plan without mutation.
- [x] B063b | B063a,B061b | `internal/sync/**`, `tests/sync/**` | Sync applies one validated field-change operation.
- [x] B064a | B063b | `internal/sync/**`, `tests/sync/**` | Archive paths and unresolved refs fail before mutation.
- [x] B064b | B064a | `internal/sync/**`, `tests/sync/**` | Stale state and invalid transitions fail before mutation.

## Mirror, CLI, Archive, Board

- [x] B070a | B011,B014 | `internal/mirror/**`, `tests/mirror/**` | Model explicit issue imports with explainable membership reason.
- [x] B070b | B070a | `internal/mirror/**`, `tests/mirror/**` | Model named scope membership alongside explicit imports.
- [x] B071a | B070b | `internal/mirror/**`, `tests/mirror/**` | Resolved out-of-scope issues become archive-eligible.
- [x] B071b | B071a | `internal/mirror/**`, `tests/mirror/**` | Pinned or unsynced issues remain live.
- [x] B080a | B015b,B016a | `internal/cli/**`, `tests/cli/**` | `jirafs use --project` updates remembered project.
- [x] B080b | B080a,B016b | `internal/cli/**`, `tests/cli/**` | `jirafs use` interactive and non-interactive selection behavior matches docs.
- [x] B081a | B052a,B053a,B070b | `internal/cli/**`, `tests/cli/**` | `jirafs mirror refresh` resolves project context and calls refresh service interface.
- [x] B081b | B081a,B053b | `internal/cli/**`, `tests/cli/**` | `jirafs mirror refresh` reports deterministic changed issue keys.
- [x] B082a | B071a | `internal/cli/**`, `tests/cli/**` | `jirafs mirror archive-sweep` reports eligible actions without mutation.
- [x] B082b | B082a | `internal/cli/**`, `tests/cli/**` | `jirafs mirror archive-sweep --apply` calls archive service interface.
- [ ] B090a | B064a,B071a | `internal/archive/**`, `tests/archive/**` | Archive movement preserves issue snapshot files.
- [ ] B090b | B090a,B071b | `internal/archive/**`, `tests/archive/**` | Archive movement preserves live membership rules.
- [ ] B091a | B052a,B070b | `internal/board/**`, `tests/board/**` | Board groups mirror issues by status.
- [ ] B091b | B091a,B052b | `internal/board/**`, `tests/board/**` | Board groups mirror issues by assignee and epic.

Notes: packets in `docs/implementation-packets.md` are orchestrator-sized. These
tasks are builder-sized. Start with B001; B020 can run after B001. Do not start
codec before B021 or planner/sync before schema, references, and export are
tested.
