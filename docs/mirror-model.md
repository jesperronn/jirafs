# Mirror Model

## Goal

The local mirror should keep a deliberately small, useful working set of Jira
issues on disk without pretending to be a full offline copy of the Jira
instance.

The mirror model must support:

- explicit import of selected issues
- reusable named scopes such as current sprint and my issues
- safe local editing and later sync for live issues
- shallow mirroring of related data
- predictable movement from live mirror to archive

## Core Idea

`jirafs` maintains two issue sets:

- a live mirror for issues that are currently in working scope
- an archive for historical snapshots that are no longer in live scope

The mirror is membership-driven, not dump-driven. An issue exists in the live
mirror because it was explicitly selected or because it currently belongs to at
least one named mirror scope.

## Selected Issue Import

The smallest unit of import is a selected issue.

Selected issue import means:

- the user can import a specific issue key directly
- the imported issue becomes part of the live mirror immediately
- direct import does not require enabling a broad board or project mirror

This is the escape hatch for one-off work. It should be possible to bring in
`ABC-123` without also importing every issue related to `ABC`.

Selected imports should be tracked as explicit membership so the system can
distinguish:

- issues present because the user asked for them
- issues present because a named scope currently resolves to them

## Named Mirror Scopes

Named mirror scopes are saved, human-readable selectors for live work.

Examples:

- `current-sprint`
- `my-issues`
- `backlog`
- `next-sprint`

Each scope has:

- a stable local name
- a resolution rule against Jira data
- optional limits such as project, board, assignee, or status filters

Examples of expected behavior:

- `current-sprint` resolves to issues in the active sprint for a configured
  board or project
- `my-issues` resolves to issues assigned to the current user, typically
  excluding fully resolved work unless configured otherwise

Scope names are part of the local model. The implementation may resolve them
through JQL, Agile board queries, or another Jira API strategy, but the user
should interact with stable local names rather than raw remote query strings in
day-to-day use.

## Mirror Membership

An issue belongs in the live mirror when at least one of these is true:

- it was explicitly imported by key
- it currently matches one or more named mirror scopes
- it is a locally created draft that has not yet been synced

Membership should be materialized locally so the system can explain why an
issue is present. For each live issue, the mirror should be able to answer:

- which scopes currently include it
- whether it was explicitly pinned or selected
- whether it is only retained because of a draft or unsynced local change

Issues should leave the live mirror when all of the following are true:

- they are not explicitly pinned or selected anymore
- they no longer match any live scope
- they have no unsynced local edits that still need user action

## Shallow Mirror Behavior

The live mirror should be shallow by default.

Shallow means:

- import the issue itself in full editable form
- mirror referenced entities as typed references and registry data
- do not recursively import every linked, parent, child, epic, or related issue
  unless that issue independently becomes a member of the live mirror

Examples:

- if `ABC-123` links to `ABC-900`, the mirror stores the link reference, but it
  does not automatically create `jira/live/issues/ABC-900.md`
- if an issue belongs to a sprint, the sprint is mirrored through registry data
  and membership metadata, not by copying the entire sprint issue set unless a
  sprint scope is active

This keeps the local workspace small and prevents accidental expansion into a
full project dump.

## Backfill of Future Sprints and Backlog

The mirror should support planned work, not only active work.

That means named scopes may intentionally backfill issues from:

- future sprints
- the backlog

Backfill is still scope-driven and shallow.

Recommended behavior:

- `current-sprint` mirrors the active sprint only
- `next-sprint` mirrors the next planned sprint when configured
- `backlog` mirrors a bounded subset of backlog issues rather than the entire
  backlog by default

Backlog and future-sprint scopes should support explicit limits so the mirror
remains workable. Suitable limits include:

- maximum issue count
- rank window
- project filter
- assignee filter
- label or component filter

The purpose of backfill is to stage likely upcoming work locally, not to create
an unbounded historical cache.

## Refresh, Sync, and Archive

The mirror lifecycle should be split into three distinct phases.

### Refresh

Refresh is a read-heavy phase that updates the local mirror from Jira and
recomputes membership.

Refresh should:

- resolve named scopes against current Jira state
- import newly matching issues into the live mirror
- update mirrored fields for live issues that have no blocking local conflict
- refresh registry data needed for references and scope resolution
- mark issues that have left all live scopes for later archive evaluation

Refresh should not push local edits to Jira.

### Sync

Sync is a write phase from local state back to Jira.

Sync should:

- inspect live issues with local edits or local drafts
- plan and apply allowed Jira write operations
- update sync metadata after successful writes
- leave archive placement unchanged except for metadata needed by later phases

Sync should not decide broad mirror membership on its own. It operates on the
current live set and the local changes within it.

### Archive

Archive is a local reclassification phase.

Archive should:

- move eligible issues out of the live mirror into archive storage
- preserve the last useful snapshot and metadata for later analysis
- avoid touching Jira except for any reads needed to confirm archive rules

Archive is about local retention policy, not remote synchronization.

## Archive Sweep Rules

Archive sweep runs after refresh and any needed sync work.

An issue should be eligible for archive sweep when all of the following are
true:

- the issue is resolved according to Jira status or resolution semantics
- the issue no longer matches any live mirror scope
- the issue is not explicitly pinned or selected for continued local presence
- the issue has no unsynced local edits or unresolved sync conflicts

When an issue is eligible:

- remove it from `jira/live/issues/`
- write or move it to the archive location as a historical snapshot
- keep enough metadata to explain when and why it left the live mirror

An issue should remain live, even if resolved, when any of these is true:

- it still matches a live scope
- the user explicitly pinned it
- it has unresolved local changes, sync conflicts, or follow-up review needs

This prevents recently completed work from disappearing before the user has
finished syncing or reviewing it.

## Practical Model

The intended working model is:

1. import a few issues explicitly or enable named scopes such as
   `current-sprint` and `my-issues`
2. run refresh to pull current remote state and update mirror membership
3. edit live issues locally and run sync to push approved changes back to Jira
4. run archive sweep to clear resolved issues that have fallen out of live scope

That yields a mirror that behaves like a focused working set with durable
history, rather than a fragile full clone of Jira.
