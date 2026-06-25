# Task: B004d - Bash-test clean-worktree guard and default `main` target behavior

## Scope:
- `bin/integrate_stream_commit`, `tests/**`

## Acceptance:
- Bash-test properly validates clean-worktree guard rejects dirty worktrees
- Bash-test verifies default target behavior uses `main` when no target specified
- Bash-test confirms explicit target behavior works correctly

## Validation:
- bin/test: PASS (shell tests)
- bin/lint: PASS
- other: Integration test structure passes

## Files changed:
- tests/fixtures/test_integrate_stream_commit.sh

## Commit:
- beb1893 test(integrate_stream_commit): add bash test for clean-worktree guard and default target behavior

## Status:
- done

## Risks:
- None