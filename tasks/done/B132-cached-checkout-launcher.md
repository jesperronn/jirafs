# B132: Run the current checkout through Go's build cache

## Scope

Add a symlink-safe `bin/jirafs` launcher that calls the existing `bin/build` on
every invocation, publishes its cached executable atomically, and preserves
application arguments, exit status and caller working directory. Build output
belongs on stderr. A failed build must not execute the previous executable.

Add explicit `bin/install --link` while retaining the default binary snapshot.
Require `--force` for conflicting links or replacing a binary with a launcher
link; never follow a replaced link or overwrite the checkout's tracked launcher.
Keep cached binaries ignored and retain the pinned connector vendor tree.

## Acceptance

- Offline shell fixtures exercise relative/absolute symlink chains, spaces and
  empty arguments, clean stdout, exit status, atomic build failure and cleanup.
- Real Go fixtures observe edits, added/deleted sources, changed embedded files
  and a missing cached executable with an otherwise warm Go cache.
- Installation fixtures cover snapshots, idempotent links, explicit replacement,
  symlinks to files/directories and protection of the tracked launcher.
- README and `bin/workflows` expose the two installation choices.
- Full `bin/lint`, `bin/test` and post-commit `bin/handoff` pass.
- No live Jira calls or writes.

## Verification

Full offline `bin/lint` and `bin/test` passed: six shell fixture suites,
36 Python checks, Go package checks and the empty-HOME/dead-proxy isolation
proof. No live Jira calls or writes. The committed branch must pass
`bin/handoff` before integration.
