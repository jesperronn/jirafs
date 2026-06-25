package board

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
	"github.com/stretchr/testify/assert"
)

func TestBoard_GroupByEpic(t *testing.T) {
	b := NewBoard()
	
	// Create test issues with different epics
	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "EPIC-1",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-2",
				Type:    "bug",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "EPIC-2",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-3",
				Type:    "task",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "",
		},
	}
	
	b.GroupByEpic(issues)
	
	// Check that we have groups for all epics
	assert.Len(t, b.EpicGroups["EPIC-1"], 1)
	assert.Len(t, b.EpicGroups["EPIC-2"], 1)
	
	// Issues without epics should go to "No Epic"
	assert.Len(t, b.EpicGroups["No Epic"], 1)
}