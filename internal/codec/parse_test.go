package codec

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
	"github.com/stretchr/testify/assert"
)

func TestParseIssue(t *testing.T) {
	t.Run("parse synced issue with all fields", func(t *testing.T) {
		content := `---
key: "ABC-123"
type: "story"
project: "project://PROJ"
schema_version: "1.0"
remote_version: "v1"
content_hash: "abc123"
state: "synced"
resolved_status: "Done"
pinned: true
summary: "Test issue summary"
description: "Test issue description"
labels:
  - "label1"
  - "label2"
assignee: "john.doe"
status: "In Progress"
sprint: "Sprint 1"
fix_versions:
  - "v1.0"
  - "v1.1"
linked_issues:
  - key: "DEF-456"
    type: "blocks"
---
## Description
This is the description content.

## Acceptance Criteria
This is the acceptance criteria content.

## Definition of Ready
This is the definition of ready content.
`

		issue, err := ParseIssue(content)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Check identity fields
		assert.Equal(t, schema.IssueKey("ABC-123"), issue.Identity.Key)
		assert.Equal(t, schema.IssueType("story"), issue.Identity.Type)
		assert.Equal(t, "project://PROJ", issue.Identity.Project.String())

		// Check machine-owned fields
		assert.Equal(t, "1.0", issue.MachineOwned.SchemaVersion)

		// Check remote metadata
		assert.Equal(t, "v1", issue.RemoteMetadata.RemoteVersion)
		assert.Equal(t, "abc123", issue.RemoteMetadata.ContentHash)
		assert.Equal(t, "synced", issue.RemoteMetadata.StateFile)
		assert.Equal(t, "Done", issue.RemoteMetadata.ResolvedStatus)
		assert.True(t, issue.RemoteMetadata.Pinned)

		// Check editable fields
		assert.Equal(t, "Test issue summary", issue.Summary)
		assert.Equal(t, "Test issue description", issue.Description)
		assert.Equal(t, []string{"label1", "label2"}, issue.Labels)
		assert.Equal(t, "john.doe", *issue.Assignee)
		assert.Equal(t, "In Progress", issue.Status)
		assert.Equal(t, "Sprint 1", issue.Sprint)
		assert.Equal(t, []string{"v1.0", "v1.1"}, issue.FixVersions)

		// Check linked issues
		assert.Len(t, issue.LinkedIssues, 1)
		assert.Equal(t, schema.IssueKey("DEF-456"), issue.LinkedIssues[0].Key)
		assert.Equal(t, "blocks", issue.LinkedIssues[0].Type)

		// Check sections
		assert.Len(t, issue.Sections, 3)
		assert.Equal(t, "This is the description content.", issue.Sections[schema.SecDescription])
		assert.Equal(t, "This is the acceptance criteria content.", issue.Sections[schema.SecAcceptanceCriteria])
		assert.Equal(t, "This is the definition of ready content.", issue.Sections[schema.SecDefinitionOfReady])
	})

	t.Run("parse draft issue with minimal fields", func(t *testing.T) {
		content := `---
key: "ABC-456"
type: "task"
project: "project://PROJ"
schema_version: "1.0"
---
## Description
This is a draft issue description.
`

		issue, err := ParseIssue(content)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Check identity fields
		assert.Equal(t, schema.IssueKey("ABC-456"), issue.Identity.Key)
		assert.Equal(t, schema.IssueType("task"), issue.Identity.Type)
		assert.Equal(t, "project://PROJ", issue.Identity.Project.String())

		// Check machine-owned fields
		assert.Equal(t, "1.0", issue.MachineOwned.SchemaVersion)

		// Check that other fields are zero values
		assert.Equal(t, "", issue.Summary)
		assert.Equal(t, "", issue.Description)
		assert.Empty(t, issue.Labels)
		assert.Nil(t, issue.Assignee)
		assert.Equal(t, "", issue.Status)
		assert.Equal(t, "", issue.Sprint)
		assert.Empty(t, issue.FixVersions)
		assert.Empty(t, issue.LinkedIssues)

		// Check sections
		assert.Len(t, issue.Sections, 1)
		assert.Equal(t, "This is a draft issue description.", issue.Sections[schema.SecDescription])
	})

	t.Run("parse issue without frontmatter", func(t *testing.T) {
		content := `## Description
This is the description content.

## Acceptance Criteria
This is the acceptance criteria content.
`

		issue, err := ParseIssue(content)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Should parse sections even without frontmatter
		assert.Len(t, issue.Sections, 2)
		assert.Equal(t, "This is the description content.", issue.Sections[schema.SecDescription])
		assert.Equal(t, "This is the acceptance criteria content.", issue.Sections[schema.SecAcceptanceCriteria])
	})

	t.Run("parse issue with unknown section", func(t *testing.T) {
		content := `---
key: "ABC-789"
type: "story"
project: "project://PROJ"
schema_version: "1.0"
---
## Description
This is the description content.

## Unknown Section
This should cause an error.
`

		_, err := ParseIssue(content)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown section name: Unknown Section")
	})
}