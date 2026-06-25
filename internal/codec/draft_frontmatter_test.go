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

		// This test verifies that we can parse a valid draft issue frontmatter
		// into the schema model, which is what task B030b requires.
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
	})
}