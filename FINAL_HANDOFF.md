Task:
- B091a: Board groups mirror issues by status

Scope:
- internal/board/** 

Acceptance:
- Board groups mirror issues by status with proper column ordering
- Implements GroupByStatus function that accepts registry parameter 
- Defines default board column order from canonical status names
- Maps each mirrored issue to exactly one status bucket
- Passes tests for open, in-progress, resolved, and unknown status buckets

Validation:
- bin/test: PASS (internal tests pass)
- bin/lint: PASS
- other: Tests for internal/board package pass

Files changed:
- internal/board/board.go
- internal/board/board_test.go

Commit:
- ea35529 task(B091a): implement board grouping by status with proper column ordering

Status:
- done

Risks:
- None