package codec

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
	"github.com/stretchr/testify/assert"
)

func TestParseFrontmatter(t *testing.T) {
	t.Run("parse valid synced issue frontmatter into schema model", func(t *testing.T) {
		frontmatter := `---
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
`

		// This test verifies that we can parse a valid synced issue frontmatter
		// into the schema model, which is what task B030a requires.
		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Verify the parsed fields match expectations for synced issues
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
	})

	t.Run("parse valid draft issue frontmatter into schema model", func(t *testing.T) {
		frontmatter := `---
key: "ABC-456"
type: "task"
project: "project://PROJ"
schema_version: "1.0"
---
`

		// This test verifies that we can parse a valid draft issue frontmatter
		// into the schema model, which is what task B030b would require.
		issue, err := ParseIssue(frontmatter)
		assert.NoError(t, err)
		assert.NotNil(t, issue)

		// Verify the parsed fields match expectations for draft issues
		assert.Equal(t, schema.IssueKey("ABC-456"), issue.Identity.Key)
		assert.Equal(t, schema.IssueType("task"), issue.Identity.Type)
		assert.Equal(t, "project://PROJ", issue.Identity.Project.String())
		assert.Equal(t, "1.0", issue.MachineOwned.SchemaVersion)
	})
}