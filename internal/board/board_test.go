package board

import (
	"testing"

	"github.com/jirafs/jirafs/internal/registry"
	"github.com/jirafs/jirafs/internal/schema"
	"github.com/stretchr/testify/assert"
)

func TestBoard_GroupByStatus(t *testing.T) {
	board := NewBoard()

	statuses := map[string]registry.Status{
		"status:open":          {Name: "Open"},
		"status:in-progress":   {Name: "In Progress"},
		"status:resolved":      {Name: "Resolved"},
		"status:closed":        {Name: "Closed"},
	}

	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "Open",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-2",
				Type:    "bug",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "In Progress",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-3",
				Type:    "task",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "Resolved",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-4",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "status:closed",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-5",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "",
		},
	}

	board.GroupByStatus(issues, statuses)

	// "Open" should have PROJ-1 (status is a raw name matching registry)
	assert.Len(t, board.StatusColumns["Open"], 1)

	// "In Progress" should have PROJ-2
	assert.Len(t, board.StatusColumns["In Progress"], 1)

	// "Resolved" should have PROJ-3
	assert.Len(t, board.StatusColumns["Resolved"], 1)

	// "Closed" should have PROJ-4 (resolved via typed ref)
	assert.Len(t, board.StatusColumns["Closed"], 1)

	// Empty status goes to "Unassigned"
	assert.Len(t, board.StatusColumns["Unassigned"], 1)

	// Column order should be sorted by status name from registry
	assert.Equal(t, []string{"Closed", "In Progress", "Open", "Resolved"}, board.ColumnOrder)
}

func TestBoard_GroupByStatus_Empty(t *testing.T) {
	b := NewBoard()

	statuses := map[string]registry.Status{
		"status:open": {Name: "Open"},
	}

	b.GroupByStatus([]*schema.Issue{}, statuses)

	assert.Empty(t, b.StatusColumns)
	assert.Equal(t, []string{"Open"}, b.ColumnOrder)
}

func TestBoard_GroupByStatus_UnknownStatus(t *testing.T) {
	b := NewBoard()

	statuses := map[string]registry.Status{
		"status:open": {Name: "Open"},
	}

	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "Nonexistent Status",
		},
	}

	b.GroupByStatus(issues, statuses)

	// Not found in registry — raw value is used as key
	assert.Len(t, b.StatusColumns["Nonexistent Status"], 1)
}

func TestBoard_NewBoard(t *testing.T) {
	board := NewBoard()

	assert.NotNil(t, board)
	assert.NotNil(t, board.StatusColumns)
	assert.NotNil(t, board.ColumnOrder)
}

func TestResolveStatusName(t *testing.T) {
	statuses := map[string]registry.Status{
		"status:in-progress": {Name: "In Progress"},
		"status:done":        {Name: "Done"},
	}

	// Empty status → "Unassigned"
	assert.Equal(t, "Unassigned", resolveStatusName("", statuses))

	// Typed ref resolves to name
	assert.Equal(t, "In Progress", resolveStatusName("status:in-progress", statuses))

	// Typed ref not in registry → raw value
	assert.Equal(t, "status:unknown", resolveStatusName("status:unknown", statuses))

	// Raw name not in registry → raw value
	assert.Equal(t, "Raw Status", resolveStatusName("Raw Status", statuses))
}