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

## Foundation

- [x] B001 | none | `go.mod`, `cmd/jirafs/**`, `internal/**`, `tests/**`, `bin/**` | Go module, `jirafs` builds, `bin/test` and `bin/lint` pass.
- [x] B002 | B001 | `cmd/jirafs/**`, `internal/cli/**`, `tests/cli/**` | Help lists `init`, `export`, `plan`, `sync`, `new`, `registry`, `board`, `archive`.
- [x] B003 | B001 | `tests/**`, `internal/testutil/**` | Fixture loading and golden-output diff helpers exist.

## Settings And Context

- [x] B010 | B001 | `internal/config/**`, `tests/config/**` | Settings errors expose stable codes and messages.
- [x] B011 | B010 | `internal/config/**`, `tests/config/**` | Parse `~/.jirafs/settings.toml`; load instances/projects; expand paths.
- [x] B012 | B011 | `internal/config/**`, `tests/config/**` | Reject duplicate keys, missing instances, bad project refs, bad mirror dirs.
- [x] B013 | B011 | `internal/context/**`, `tests/context/**` | Explicit `--project` beats every other source.
- [x] B014 | B013 | `internal/context/**`, `tests/context/**` | Cwd mapping uses most-specific match; ambiguity fails clearly.
- [ ] B015 | B013 | `internal/context/**`, `tests/context/**` | Remembered current project is read/written and lower precedence.
- [ ] B016 | B013 | `internal/context/**`, `tests/context/**` | Non-interactive unresolved context fails without prompting.

## Schema

- [ ] B020 | B001 | `internal/schema/**`, `tests/schema/**` | One typed-ref representation for users, projects, statuses, sprints, versions, epics, issues.
- [ ] B021 | B020 | `internal/schema/**`, `tests/schema/**` | Issue model covers required frontmatter and fixed sections from `docs/issue-format.md`.
- [ ] B022 | B021 | `internal/schema/**`, `tests/schema/**` | Sync metadata validates remote version, hash, timestamps, syncable state.
- [ ] B023 | B020 | `internal/schema/**`, `tests/schema/**` | Registry models match `docs/references.md`.
- [ ] B024 | B021 | `internal/schema/**`, `tests/schema/**` | Plan/operation models type editable changes and conflicts without Jira transport.

## Codec

- [ ] B030 | B021 | `internal/codec/**`, `tests/codec/**` | Parse synced/draft frontmatter; invalid frontmatter gives structured errors.
- [ ] B031 | B030 | `internal/codec/**`, `tests/codec/**` | Parse known sections; unknown sections fail.
- [ ] B032 | B031 | `internal/codec/**`, `tests/codec/**` | Render issue docs with stable field and section order.
- [ ] B033 | B032 | `tests/codec/**` | Synced and draft fixtures are stable on second render.

## Registry And References

- [ ] B040 | B011,B023 | `internal/registry/**`, `tests/registry/**` | Load each registry family with structured file errors.
- [ ] B041 | B040 | `internal/references/**`, `tests/references/**` | Resolve all typed refs to Jira ids through registries.
- [ ] B042 | B041 | `internal/references/**`, `tests/references/**` | Missing/ambiguous refs include type, lookup value, candidates when available.

## Jira Read Path

- [ ] B050 | B021 | `internal/jira/**`, `tests/jira/**` | Jira client interface and fake transport allow JSON tests without network.
- [ ] B051 | B050 | `internal/jira/**`, `tests/jira/**` | Fetch one issue payload into normalized remote data.
- [ ] B052 | B021,B051 | `internal/export/**`, `tests/export/**` | Normalize one remote issue into canonical issue model; links stay shallow refs.
- [ ] B053 | B050 | `internal/jira/**`, `tests/jira/**` | Fake `my-issues` or `current-sprint` search returns deterministic issue keys.

## Planner And Sync

- [ ] B060 | B024,B052 | `internal/plan/**`, `tests/plan/**` | Unchanged local/remote models produce empty typed plan.
- [ ] B061 | B060 | `internal/plan/**`, `tests/plan/**` | Editable fields become typed operations.
- [ ] B062 | B060 | `internal/plan/**`, `tests/plan/**` | Stale version/hash produces conflicts, not operations.
- [ ] B063 | B061 | `internal/sync/**`, `tests/sync/**` | Sync accepts only validated plan objects.
- [ ] B064 | B063 | `internal/sync/**`, `tests/sync/**` | Archive paths, unresolved refs, stale state, invalid transitions fail before mutation.

## Mirror, CLI, Archive, Board

- [ ] B070 | B011,B014 | `internal/mirror/**`, `tests/mirror/**` | Explicit imports and named scopes coexist with explainable membership.
- [ ] B071 | B070 | `internal/mirror/**`, `tests/mirror/**` | Resolved out-of-scope issues can archive; pinned/unsynced stay live.
- [ ] B080 | B015,B016 | `internal/cli/**`, `tests/cli/**` | `jirafs use` matches interactive/non-interactive project selection docs.
- [ ] B081 | B052,B053,B070 | `internal/cli/**`, `tests/cli/**` | `jirafs mirror refresh` uses project context and service interfaces.
- [ ] B082 | B071 | `internal/cli/**`, `tests/cli/**` | `jirafs mirror archive-sweep` reports actions; mutates only when requested.
- [ ] B090 | B064,B071 | `internal/archive/**`, `tests/archive/**` | Archive movement preserves snapshots and live membership rules.
- [ ] B091 | B052,B070 | `internal/board/**`, `tests/board/**` | Board groups mirror issues by status, assignee, epic.

Notes: packets in `docs/implementation-packets.md` are orchestrator-sized. These
tasks are builder-sized. Start with B001; B020 can run after B001. Do not start
codec before B021 or planner/sync before schema, references, and export are
tested.
