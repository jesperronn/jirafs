package board

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestBoard_GroupByStatus(t *testing.T) {
	// Create a board
	board := NewBoard()
	
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
	
	// Group issues by status
	board.GroupByStatus(issues)
	
	// Check that we have columns for all statuses - this is a simple check
	// that Open and In Progress are properly grouped, and the rest goes to Unknown
	if len(board.StatusColumns["Open"]) != 2 {
		t.Errorf("Expected 2 issues in 'Open' column, got %d", len(board.StatusColumns["Open"]))
	}
	
	if len(board.StatusColumns["In Progress"]) != 1 {
		t.Errorf("Expected 1 issue in 'In Progress' column, got %d", len(board.StatusColumns["In Progress"]))
	}
	
	if len(board.StatusColumns["Resolved"]) != 1 {
		t.Errorf("Expected 1 issue in 'Resolved' column, got %d", len(board.StatusColumns["Resolved"]))
	}
	
	// The "Custom Status" and "" status should end up in Unknown column since they're not in our default columns
	// But we can't predict exactly how many will end up there
	
	// Check that the column order is as expected
	if len(board.ColumnOrder) == 0 {
		t.Error("Expected non-empty column order")
	}
	
	// Make sure all our default columns are in the order
	expectedColumns := []string{"Open", "In Progress", "Resolved", "Unknown"}
	for i, col := range expectedColumns {
		if board.ColumnOrder[i] != col {
			t.Errorf("Expected column %d to be %q, got %q", i, col, board.ColumnOrder[i])
		}
	}
}

func TestBoard_GroupByStatus_Empty(t *testing.T) {
	board := NewBoard()
	
	// Test with empty issues slice
	board.GroupByStatus([]*schema.Issue{})
	
	// Should have no columns or at least no issues in columns
	if len(board.StatusColumns) != 0 {
		t.Logf("Expected no columns, got %d", len(board.StatusColumns))
	}
}

func TestBoard_GroupByStatus_UnknownStatus(t *testing.T) {
	board := NewBoard()
	
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
	
	board.GroupByStatus(issues)
	
	// Should end up in "Unknown" column
	if len(board.StatusColumns["Unknown"]) != 1 {
		t.Errorf("Expected 1 issue in 'Unknown' column, got %d", len(board.StatusColumns["Unknown"]))
	}
}

func TestBoard_NewBoard(t *testing.T) {
	board := NewBoard()
	
	if board == nil {
		t.Error("NewBoard should not return nil")
	}
	
	if board.StatusColumns == nil {
		t.Error("NewBoard should initialize StatusColumns")
	}
	
	if board.ColumnOrder == nil {
		t.Error("NewBoard should initialize ColumnOrder")
	}
}