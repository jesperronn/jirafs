Task:
- B030a: Parse valid synced issue frontmatter into schema model

Scope:
- internal/codec/**

Acceptance:
- The codec package now contains a ParseIssue function that can parse valid synced issue frontmatter into the schema model.
- The parser handles all fields in the schema including identity, machine-owned, remote metadata, editable fields, and linked issues.
- The parser can also parse issue bodies into sections.
- The parser validates that section names are known fixed sections and rejects unknown ones.

Validation:
- bin/test: PASS
- bin/lint: PASS
- other: The codec package tests all pass

Files changed:
- internal/codec/parse.go
- internal/codec/parse_test.go

Commit:
- db9e5f0 task(B030a): add parser for valid synced issue frontmatter into schema model

Status:
- done

Risks:
- None