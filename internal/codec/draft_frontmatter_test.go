package codec

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
	"github.com/stretchr/testify/assert"
)

func TestParseDraftFrontmatter(t *testing.T) {
	t.Run("parse valid draft issue frontmatter into schema model", func(t *testing.T) {
		frontmatter := `---
key: "ABC-456"
type: "task"
project: "project://PROJ"
schema_version: "1.0"
---
`

		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Verify the parsed fields match expectations for draft issues
		assert.Equal(t, schema.IssueKey("ABC-456"), issue.Identity.Key)
		assert.Equal(t, schema.IssueType("task"), issue.Identity.Type)
		assert.Equal(t, "project://PROJ", issue.Identity.Project.String())
		assert.Equal(t, "1.0", issue.MachineOwned.SchemaVersion)

		// Verify that all other fields are zero values for a draft issue
		assert.Equal(t, "", issue.Summary)
		assert.Equal(t, "", issue.Description)
		assert.Empty(t, issue.Labels)
		assert.Nil(t, issue.Assignee)
		assert.Equal(t, "", issue.Status)
		assert.Equal(t, "", issue.Sprint)
		assert.Empty(t, issue.FixVersions)
		assert.Empty(t, issue.LinkedIssues)

		// Verify RemoteMetadata is empty for draft issues
		assert.Equal(t, "", issue.RemoteMetadata.RemoteVersion)
		assert.Equal(t, "", issue.RemoteMetadata.ContentHash)
		assert.Equal(t, "", issue.RemoteMetadata.StateFile)
		assert.Equal(t, "", issue.RemoteMetadata.ResolvedStatus)
		assert.False(t, issue.RemoteMetadata.Pinned)
	})

	t.Run("parse valid draft issue with all editable fields", func(t *testing.T) {
		frontmatter := `---
key: "ABC-789"
type: "story"
project: "project://MYPROJ"
schema_version: "1.0"
summary: "Draft story summary"
description: "Draft story description"
labels:
  - "urgent"
  - "frontend"
assignee: "jane.doe"
status: "To Do"
sprint: "Sprint 42"
fix_versions:
  - "v2.0"
  - "v2.1"
linked_issues:
  - key: "DEF-100"
    type: "blocks"
  - key: "DEF-200"
    type: "relates to"
---
`

		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Verify identity fields
		assert.Equal(t, schema.IssueKey("ABC-789"), issue.Identity.Key)
		assert.Equal(t, schema.IssueType("story"), issue.Identity.Type)
		assert.Equal(t, "project://MYPROJ", issue.Identity.Project.String())
		assert.Equal(t, "1.0", issue.MachineOwned.SchemaVersion)

		// Verify editable fields
		assert.Equal(t, "Draft story summary", issue.Summary)
		assert.Equal(t, "Draft story description", issue.Description)
		assert.Equal(t, []string{"urgent", "frontend"}, issue.Labels)
		assert.NotNil(t, issue.Assignee)
		assert.Equal(t, "jane.doe", *issue.Assignee)
		assert.Equal(t, "To Do", issue.Status)
		assert.Equal(t, "Sprint 42", issue.Sprint)
		assert.Equal(t, []string{"v2.0", "v2.1"}, issue.FixVersions)

		// Verify linked issues
		assert.Len(t, issue.LinkedIssues, 2)
		assert.Equal(t, schema.IssueKey("DEF-100"), issue.LinkedIssues[0].Key)
		assert.Equal(t, "blocks", issue.LinkedIssues[0].Type)
		assert.Equal(t, schema.IssueKey("DEF-200"), issue.LinkedIssues[1].Key)
		assert.Equal(t, "relates to", issue.LinkedIssues[1].Type)

		// Verify RemoteMetadata is empty for draft issues
		assert.Equal(t, "", issue.RemoteMetadata.RemoteVersion)
		assert.Equal(t, "", issue.RemoteMetadata.ContentHash)
	})

	t.Run("parse draft issue with sections", func(t *testing.T) {
		content := `---
key: "ABC-999"
type: "bug"
project: "project://BUGPROJ"
schema_version: "1.0"
summary: "Critical bug in login"
---
## Description
The login page crashes when username contains special characters.

## Acceptance Criteria
- Login works with alphanumeric usernames
- Login works with usernames containing @ and .
- Proper error message shown for invalid characters
`

		issue, err := ParseIssue(content)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		assert.Equal(t, schema.IssueKey("ABC-999"), issue.Identity.Key)
		assert.Equal(t, schema.IssueType("bug"), issue.Identity.Type)
		assert.Equal(t, "Critical bug in login", issue.Summary)

		// Verify sections are parsed
		assert.Len(t, issue.Sections, 2)
		assert.Equal(t, "The login page crashes when username contains special characters.",
			issue.Sections[schema.SecDescription])
		assert.Equal(t, "- Login works with alphanumeric usernames\n- Login works with usernames containing @ and .\n- Proper error message shown for invalid characters",
			issue.Sections[schema.SecAcceptanceCriteria])
	})

	t.Run("parse draft issue with partial editable fields", func(t *testing.T) {
		frontmatter := `---
key: "ABC-111"
type: "task"
project: "project://PARTIAL"
schema_version: "1.0"
summary: "Only summary set"
labels:
  - "partial"
---
`

		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		assert.Equal(t, schema.IssueKey("ABC-111"), issue.Identity.Key)
		assert.Equal(t, "Only summary set", issue.Summary)
		assert.Equal(t, []string{"partial"}, issue.Labels)

		// Other editable fields should be zero values
		assert.Equal(t, "", issue.Description)
		assert.Nil(t, issue.Assignee)
		assert.Equal(t, "", issue.Status)
		assert.Equal(t, "", issue.Sprint)
		assert.Empty(t, issue.FixVersions)
		assert.Empty(t, issue.LinkedIssues)
	})

	t.Run("parse draft issue with empty arrays", func(t *testing.T) {
		frontmatter := `---
key: "ABC-222"
type: "story"
project: "project://EMPTY"
schema_version: "1.0"
labels: []
fix_versions: []
linked_issues: []
---
`

		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		assert.Equal(t, schema.IssueKey("ABC-222"), issue.Identity.Key)
		// Empty arrays should parse as empty slices, not nil
		assert.Empty(t, issue.Labels)
		assert.Empty(t, issue.FixVersions)
		assert.Empty(t, issue.LinkedIssues)
	})

	t.Run("parse draft issue with no frontmatter", func(t *testing.T) {
		content := `This is a document with no frontmatter.

## Description
Some description content.
`

		issue, err := ParseIssue(content)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Should parse sections even without frontmatter
		assert.Len(t, issue.Sections, 1)
		assert.Equal(t, "Some description content.", issue.Sections[schema.SecDescription])
		// Identity should be zero
		assert.True(t, issue.Identity.IsZero())
	})

	t.Run("draft issue IsZero reports true when no identity or machine fields", func(t *testing.T) {
		frontmatter := `---
---
`

		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)
		assert.True(t, issue.IsZero())
	})

	t.Run("draft issue IsZero reports false with identity", func(t *testing.T) {
		frontmatter := `---
key: "ABC-333"
type: "task"
project: "project://TEST"
schema_version: "1.0"
---
`

		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)
		assert.False(t, issue.IsZero())
	})
}