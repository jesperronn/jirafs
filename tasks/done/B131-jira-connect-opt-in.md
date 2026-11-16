# B131: Opt in to the shared Jira adapter

## Scope

Select the consumer-local jira-connect bridge with `JIRAFS_JIRA_ADAPTER` while
keeping the existing adapter as the default and explicit rollback option.
Preserve consumer credential resolution, epic links and custom fields. Keep Git,
mirror, conflict and local file policy in jirafs.

## Acceptance

- Explicit selector, legacy default and invalid-value rejection before secrets.
- Generic connector pinned and vendored; no sibling paths or CI credentials.
- Accepted updates with failed refreshes are distinct from rejected writes;
  status transitions stop and local files/base snapshots remain unchanged.
- Unknown update/create/transition outcomes do not retry; create intents survive.
- Read-only refresh hints retain project/cwd context.
- Offline fake HTTP coverage, including legacy rejection and dependency setup.
- No live Jira writes.

## Verification

Full offline `bin/lint` and `bin/test` passed, including four shell suites,
36 Python checks and the empty-HOME/dead-proxy isolation run. No live Jira writes.
The committed feature branch must pass `bin/handoff` before integration.
