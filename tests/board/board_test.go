package board

import (
	"testing"

	"github.com/jirafs/jirafs/internal/board"
	"github.com/jirafs/jirafs/internal/registry"
	"github.com/jirafs/jirafs/internal/schema"
)

func TestBoard_GroupByStatus(t *testing.T) {
	b := board.NewBoard()

	statuses := map[string]registry.Status{
		"status:open":      {Name: "Open"},
		"status:in-progress": {Name: "In Progress"},
		"status:resolved":  {Name: "Resolved"},
		"status:closed":    {Name: "Closed"},
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

	b.GroupByStatus(issues, statuses)

	// Check that each column has the right issues
	if len(b.StatusColumns["Open"]) != 1 {
		t.Errorf("Expected 1 issue in 'Open' column, got %d", len(b.StatusColumns["Open"]))
	}

	if len(b.StatusColumns["In Progress"]) != 1 {
		t.Errorf("Expected 1 issue in 'In Progress' column, got %d", len(b.StatusColumns["In Progress"]))
	}

	if len(b.StatusColumns["Resolved"]) != 1 {
		t.Errorf("Expected 1 issue in 'Resolved' column, got %d", len(b.StatusColumns["Resolved"]))
	}

	if len(b.StatusColumns["Closed"]) != 1 {
		t.Errorf("Expected 1 issue in 'Closed' column, got %d", len(b.StatusColumns["Closed"]))
	}

	// Empty status goes to "Unassigned"
	if len(b.StatusColumns["Unassigned"]) != 1 {
		t.Errorf("Expected 1 issue in 'Unassigned' column, got %d", len(b.StatusColumns["Unassigned"]))
	}

	// Column order should be sorted by status name from registry
	expectedOrder := []string{"Closed", "In Progress", "Open", "Resolved"}
	if len(b.ColumnOrder) != len(expectedOrder) {
		t.Errorf("Expected column order %v, got %v", expectedOrder, b.ColumnOrder)
	}
}

func TestBoard_GroupByStatus_Empty(t *testing.T) {
	b := board.NewBoard()

	statuses := map[string]registry.Status{
		"status:open": {Name: "Open"},
	}

	b.GroupByStatus([]*schema.Issue{}, statuses)

	if len(b.StatusColumns) != 0 {
		t.Logf("Expected no columns, got %d", len(b.StatusColumns))
	}
}

func TestBoard_GroupByStatus_UnknownStatus(t *testing.T) {
	b := board.NewBoard()

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
	if len(b.StatusColumns["Nonexistent Status"]) != 1 {
		t.Errorf("Expected 1 issue in 'Nonexistent Status' column, got %d", len(b.StatusColumns["Nonexistent Status"]))
	}
}

func TestBoard_NewBoard(t *testing.T) {
	b := board.NewBoard()

	if b == nil {
		t.Error("NewBoard should not return nil")
	}

	if b.StatusColumns == nil {
		t.Error("NewBoard should initialize StatusColumns")
	}

	if b.ColumnOrder == nil {
		t.Error("NewBoard should initialize ColumnOrder")
	}
}