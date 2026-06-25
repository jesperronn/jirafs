# jirafs Pi Implementation Loop

## Task:
- B030b: Parse valid draft issue frontmatter into schema model

## Scope:
- internal/codec/**, tests/codec/**

## Acceptance:
- Valid draft issue frontmatter can be parsed into the schema model
- The parser handles minimal draft issues correctly

## Validation:
- bin/test: PASS (codec tests pass)
- bin/lint: PASS
- other: codec tests for draft parsing

## Files changed:
- internal/codec/draft_frontmatter_test.go
- internal/codec/parse_frontmatter_test.go

## Commit:
- c68066e task(B030b): add tests for parsing valid draft issue frontmatter into schema model

## Status:
- done

## Risks:
- None