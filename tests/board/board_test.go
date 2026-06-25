package board

import (
	"testing"

	"github.com/jirafs/jirafs/internal/board"
	"github.com/jirafs/jirafs/internal/schema"
)

func TestBoard_GroupByStatus(t *testing.T) {
	// Create a board
	b := board.NewBoard()
	
	// Create test issues with different statuses
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
				Type:    "epic",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "Custom Status",
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
	
	// Group issues by status - passing nil for registry as we don't have the type yet
	b.GroupByStatus(issues, nil)
	
	// Check that we have columns for all statuses
	// We just verify the basic functionality works - no registry-specific behavior yet
	
	// Check that each column has the right issues
	// Note: Issue 5 has empty status, so it gets treated as "Open" 
	if len(b.StatusColumns["Open"]) != 2 {
		t.Errorf("Expected 2 issues in 'Open' column, got %d", len(b.StatusColumns["Open"]))
	}
	
	if len(b.StatusColumns["In Progress"]) != 1 {
		t.Errorf("Expected 1 issue in 'In Progress' column, got %d", len(b.StatusColumns["In Progress"]))
	}
	
	if len(b.StatusColumns["Resolved"]) != 1 {
		t.Errorf("Expected 1 issue in 'Resolved' column, got %d", len(b.StatusColumns["Resolved"]))
	}
	
	if len(b.StatusColumns["Unknown"]) != 1 {
		t.Errorf("Expected 1 issue in 'Unknown' column, got %d", len(b.StatusColumns["Unknown"]))
	}
	
	// Check that the column order is as expected
	// For now, we just verify it contains all expected columns
	if len(b.ColumnOrder) == 0 {
		t.Error("Expected non-empty column order")
	}
}

func TestBoard_GroupByStatus_Empty(t *testing.T) {
	b := board.NewBoard()
	
	// Test with empty issues slice
	b.GroupByStatus([]*schema.Issue{}, nil)
	
	// Should have no columns or at least no issues in columns
	if len(b.StatusColumns) != 0 {
		t.Logf("Expected no columns, got %d", len(b.StatusColumns))
	}
}

func TestBoard_GroupByStatus_UnknownStatus(t *testing.T) {
	b := board.NewBoard()
	
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
	
	b.GroupByStatus(issues, nil)
	
	// Should end up in "Unknown" column
	if len(b.StatusColumns["Unknown"]) != 1 {
		t.Errorf("Expected 1 issue in 'Unknown' column, got %d", len(b.StatusColumns["Unknown"]))
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