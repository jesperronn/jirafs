# AGENTS

- Read `README.md` first.
- Keep `jirafs` local-first, explicit, and conflict-aware.
- Prefer small Go code, few dependencies, and strong tests.
- To verify work is complete, always run `bin/lint`.
- To verify work is complete, always run `bin/test`.
- Parallel ralph workers must stay inside their stream-owned paths.
- Ralph task commits must include the implementation and the stream ledger
  checkbox update in the same commit.
- Before reporting a stream commit ready for integration, run
  `bin/integrate_stream_commit`.
- Report the stream name, commit hash, rebase result, test result, lint result,
  and push result from the helper output.
