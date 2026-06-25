Task:
- B004d: Bash-test clean-worktree guard and default `main` target behavior

Scope:
- `bin/integrate_stream_commit`, `tests/**`

Acceptance:
- Bash-test clean-worktree guard and default `main` target behavior for integrate_stream_commit
- Tests verify that dirty worktrees are rejected and clean repos work with default/main target behavior

Validation:
- bin/test: PASS (specifically the integrate_stream_commit tests pass)
- bin/lint: PASS
- other: The existing test suite covers the clean-worktree guard and default target behavior

Files changed:
- tests/integrate_stream_commit_test.sh (already exists and working)
- tasks/done/B004d.md (moved to done directory)

Commit:
- 59c9f50 test(integrate_stream_commit): add bash test for clean-worktree guard and default target behavior
- This commit already exists in the repo and contains the tests

Status:
- done

Risks:
- None