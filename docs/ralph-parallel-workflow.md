# Ralph Parallel Workflow

Use separate git worktrees for parallel ralph streams:

- `P1-worktree`: WS1 schema and codec.
- `P2-worktree`: WS2 settings, context, registry, references.
- `P3-worktree`: WS3 Jira and export, dependency-gated until WS1 issue models
  exist.

Run each worktree with `ralph run --stop-on-error`. The ralph prompt chooses
the first ready unchecked task in that stream ledger, commits it, and stops.
When a stream has no unchecked tasks left, it should report `complete` and exit
non-zero so the Ralphify loop terminates. When tasks remain but none are ready,
it should report `blocked` and exit non-zero for the same reason.

Each stream task must produce one commit containing both:

- the implementation and tests
- the stream ledger checkbox update

After each successful stream commit, run:

```bash
bin/integrate_stream_commit
```

The helper runs `git rebase main`, `bin/test`, `bin/lint`, and
`git push . HEAD:main`. If the push fails because another stream updated
`main`, it waits a random 1-3 seconds and retries the full
rebase/test/lint/push cycle up to five times.

If rebase or validation fails, the helper stops without pushing. Fix inside the
stream worktree or report the blocker with the failing command output.

Multiple stream worktrees can develop concurrently. `main` updates are
serialized by the helper's retry loop.

## Parallel Setup

```asciidoc
[main Codex orchestrator]
  |
  |-- planlægger parallelle bidder
  |-- skriver: todo-1, todo-2
  v

        +-------------------+         +-------------------+
        |    worktree-1     |         |    worktree-2     |
        |                   |         |                   |
        | henter opgaver    |         | henter opgaver    |
        | fra todo-2        |         | fra todo-2        |
        +-------------------+         +-------------------+
                 \                             /
                  \                           /
                   \                         /
                    v                       v

                          [main]
                    merges efterhånden
```
