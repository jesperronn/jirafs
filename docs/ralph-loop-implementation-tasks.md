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
`B020b`, `B020c`.

## Foundation

- [ ] B004a | B001 | `bin/**`, `tests/**`, `tools/**` | Add bash test runner and one passing shell-test fixture wired into `bin/test`.
- [ ] B004b | B004a | `bin/test`, `tests/**` | Bash-test `bin/test` argument handling: default behavior and invalid args.
- [ ] B004c | B004a | `bin/lint`, `tests/**` | Bash-test `bin/lint` argument handling: default behavior and invalid args.
- [ ] B004d | B004a | `bin/integrate_stream_commit`, `tests/**` | Bash-test clean-worktree guard and default `main` target behavior.
- [ ] B004e | B004d | `bin/integrate_stream_commit`, `tests/**` | Bash-test retry/backoff after non-fast-forward push failure.
- [ ] B004f | B004d | `bin/integrate_stream_commit`, `tests/**` | Bash-test rebase, test, and lint failure paths stop before push.

## Settings And Context

- [ ] B015a | B013 | `internal/context/**`, `tests/context/**` | Read remembered current project when no explicit project or cwd match exists.
- [ ] B015b | B015a | `internal/context/**`, `tests/context/**` | Write remembered current project after successful explicit selection.
- [ ] B016a | B013 | `internal/context/**`, `tests/context/**` | Non-interactive unresolved context returns a structured no-project error.
- [ ] B016b | B016a | `internal/context/**`, `tests/context/**` | Interactive unresolved context selects from known projects.

## Schema

- [ ] B021a | B020c | `internal/schema/**`, `tests/schema/**` | Define issue identity and machine-owned frontmatter fields.
- [ ] B021b | B021a | `internal/schema/**`, `tests/schema/**` | Define editable issue fields and fixed section names.
- [ ] B021c | B021b | `internal/schema/**`, `tests/schema/**` | Validate required issue fields and unknown sections.
- [ ] B022a | B021a | `internal/schema/**`, `tests/schema/**` | Define remote version, content hash, and sync timestamp metadata.
- [ ] B022b | B022a | `internal/schema/**`, `tests/schema/**` | Validate syncable, unsynced, archived, and draft states.
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
- [ ] B033a | B032b | `tests/codec/**` | Add synced issue round-trip golden fixture.
- [ ] B033b | B033a | `tests/codec/**` | Add draft issue round-trip golden fixture.

## Registry And References

- [ ] B040a | B011,B023b | `internal/registry/**`, `tests/registry/**` | Load user and project registry files with structured file errors.
- [ ] B040b | B040a | `internal/registry/**`, `tests/registry/**` | Load status, sprint, and fix-version registry files.
- [ ] B041a | B040a | `internal/references/**`, `tests/references/**` | Resolve user and project typed refs to Jira ids.
- [ ] B041b | B041a,B040b | `internal/references/**`, `tests/references/**` | Resolve status, sprint, and fix-version typed refs to Jira ids.
- [ ] B042a | B041a | `internal/references/**`, `tests/references/**` | Missing refs include type and lookup value.
- [ ] B042b | B042a | `internal/references/**`, `tests/references/**` | Ambiguous refs include candidate context.

## Jira Read Path

- [ ] B050a | B021a | `internal/jira/**`, `tests/jira/**` | Define Jira client interface and request/response error type.
- [ ] B050b | B050a | `internal/jira/**`, `tests/jira/**` | Add fake transport for JSON tests without network.
- [ ] B051a | B050b | `internal/jira/**`, `tests/jira/**` | Fetch one issue payload by key into remote data.
- [ ] B051b | B051a | `internal/jira/**`, `tests/jira/**` | Map Jira fetch errors to structured client errors.
- [ ] B052a | B021b,B051a | `internal/export/**`, `tests/export/**` | Normalize summary, description, labels, and assignee into issue model.
- [ ] B052b | B052a | `internal/export/**`, `tests/export/**` | Normalize linked issues as shallow typed refs.
- [ ] B053a | B050b | `internal/jira/**`, `tests/jira/**` | Search fake `my-issues` scope with deterministic keys.
- [ ] B053b | B053a | `internal/jira/**`, `tests/jira/**` | Search fake `current-sprint` scope with deterministic keys.

## Planner And Sync

- [ ] B060a | B024b,B052a | `internal/plan/**`, `tests/plan/**` | Unchanged local/remote summary and description produce empty plan.
- [ ] B060b | B060a,B052b | `internal/plan/**`, `tests/plan/**` | Unchanged refs and metadata produce empty plan.
- [ ] B061a | B060a | `internal/plan/**`, `tests/plan/**` | Summary and description changes become typed operations.
- [ ] B061b | B061a | `internal/plan/**`, `tests/plan/**` | Labels, assignee, status, sprint, and fix-version changes become typed operations.
- [ ] B062a | B060b | `internal/plan/**`, `tests/plan/**` | Stale remote version produces conflict, not operations.
- [ ] B062b | B062a | `internal/plan/**`, `tests/plan/**` | Stale content hash produces conflict, not operations.
- [ ] B063a | B061a | `internal/sync/**`, `tests/sync/**` | Sync applies a validated no-op plan without mutation.
- [ ] B063b | B063a,B061b | `internal/sync/**`, `tests/sync/**` | Sync applies one validated field-change operation.
- [ ] B064a | B063b | `internal/sync/**`, `tests/sync/**` | Archive paths and unresolved refs fail before mutation.
- [ ] B064b | B064a | `internal/sync/**`, `tests/sync/**` | Stale state and invalid transitions fail before mutation.

## Mirror, CLI, Archive, Board

- [ ] B070a | B011,B014 | `internal/mirror/**`, `tests/mirror/**` | Model explicit issue imports with explainable membership reason.
- [ ] B070b | B070a | `internal/mirror/**`, `tests/mirror/**` | Model named scope membership alongside explicit imports.
- [ ] B071a | B070b | `internal/mirror/**`, `tests/mirror/**` | Resolved out-of-scope issues become archive-eligible.
- [ ] B071b | B071a | `internal/mirror/**`, `tests/mirror/**` | Pinned or unsynced issues remain live.
- [ ] B080a | B015b,B016a | `internal/cli/**`, `tests/cli/**` | `jirafs use --project` updates remembered project.
- [ ] B080b | B080a,B016b | `internal/cli/**`, `tests/cli/**` | `jirafs use` interactive and non-interactive selection behavior matches docs.
- [ ] B081a | B052a,B053a,B070b | `internal/cli/**`, `tests/cli/**` | `jirafs mirror refresh` resolves project context and calls refresh service interface.
- [ ] B081b | B081a,B053b | `internal/cli/**`, `tests/cli/**` | `jirafs mirror refresh` reports deterministic changed issue keys.
- [ ] B082a | B071a | `internal/cli/**`, `tests/cli/**` | `jirafs mirror archive-sweep` reports eligible actions without mutation.
- [ ] B082b | B082a | `internal/cli/**`, `tests/cli/**` | `jirafs mirror archive-sweep --apply` calls archive service interface.
- [ ] B090a | B064a,B071a | `internal/archive/**`, `tests/archive/**` | Archive movement preserves issue snapshot files.
- [ ] B090b | B090a,B071b | `internal/archive/**`, `tests/archive/**` | Archive movement preserves live membership rules.
- [ ] B091a | B052a,B070b | `internal/board/**`, `tests/board/**` | Board groups mirror issues by status.
- [ ] B091b | B091a,B052b | `internal/board/**`, `tests/board/**` | Board groups mirror issues by assignee and epic.

Notes: packets in `docs/implementation-packets.md` are orchestrator-sized. These
tasks are builder-sized. Start with B001; B020 can run after B001. Do not start
codec before B021 or planner/sync before schema, references, and export are
tested.
