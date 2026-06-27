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

func TestBoard_GroupByAssignee(t *testing.T) {
	b := board.NewBoard()

	users := map[string]registry.User{
		"user:jesper": {AccountID: "712020:abcd", DisplayName: "Jesper Ronn"},
		"user:bob":    {AccountID: "712020:efgh", DisplayName: "Bob Smith"},
	}

	assigneeJesper := "user:jesper"
	assigneeBob := "user:bob"

	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Assignee: &assigneeJesper,
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-2",
				Type:    "bug",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Assignee: &assigneeBob,
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-3",
				Type:    "task",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Assignee: nil,
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-4",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Assignee: &assigneeJesper,
		},
	}

	b.GroupByAssignee(issues, users)

	// "712020:abcd" (Jesper) should have 2 issues
	if len(b.AssigneeColumns["712020:abcd"]) != 2 {
		t.Errorf("Expected 2 issues in '712020:abcd' column, got %d", len(b.AssigneeColumns["712020:abcd"]))
	}

	// "712020:efgh" (Bob) should have 1 issue
	if len(b.AssigneeColumns["712020:efgh"]) != 1 {
		t.Errorf("Expected 1 issue in '712020:efgh' column, got %d", len(b.AssigneeColumns["712020:efgh"]))
	}

	// "Unassigned" should have 1 issue (nil assignee)
	if len(b.AssigneeColumns["Unassigned"]) != 1 {
		t.Errorf("Expected 1 issue in 'Unassigned' column, got %d", len(b.AssigneeColumns["Unassigned"]))
	}

	// Column order should be sorted by account ID, "Unassigned" last
	expectedOrder := []string{"712020:abcd", "712020:efgh", "Unassigned"}
	if len(b.ColumnOrder) != len(expectedOrder) {
		t.Errorf("Expected column order %v, got %v", expectedOrder, b.ColumnOrder)
	}

	// GroupMode should be GroupModeAssignee
	if b.GroupMode != "assignee" {
		t.Errorf("Expected GroupMode 'assignee', got %q", b.GroupMode)
	}
}

func TestBoard_GroupByAssignee_Empty(t *testing.T) {
	b := board.NewBoard()

	users := map[string]registry.User{
		"user:jesper": {AccountID: "712020:abcd"},
	}

	b.GroupByAssignee([]*schema.Issue{}, users)

	if len(b.AssigneeColumns) != 0 {
		t.Logf("Expected no columns, got %d", len(b.AssigneeColumns))
	}
}

func TestBoard_GroupByAssignee_UnresolvedUser(t *testing.T) {
	b := board.NewBoard()

	users := map[string]registry.User{
		"user:jesper": {AccountID: "712020:abcd"},
	}

	assigneeBob := "user:bob"

	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Assignee: &assigneeBob,
		},
	}

	b.GroupByAssignee(issues, users)

	// Unresolved user falls back to raw name
	if len(b.AssigneeColumns["user:bob"]) != 1 {
		t.Errorf("Expected 1 issue in 'user:bob' column, got %d", len(b.AssigneeColumns["user:bob"]))
	}
}

func TestBoard_GroupByEpic(t *testing.T) {
	b := board.NewBoard()

	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "PROJ-100",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-2",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "PROJ-200",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-3",
				Type:    "task",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "PROJ-100",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-4",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "",
		},
	}

	b.GroupByEpic(issues)

	// "PROJ-100" should have 2 issues
	if len(b.EpicColumns["PROJ-100"]) != 2 {
		t.Errorf("Expected 2 issues in 'PROJ-100' column, got %d", len(b.EpicColumns["PROJ-100"]))
	}

	// "PROJ-200" should have 1 issue
	if len(b.EpicColumns["PROJ-200"]) != 1 {
		t.Errorf("Expected 1 issue in 'PROJ-200' column, got %d", len(b.EpicColumns["PROJ-200"]))
	}

	// "Unassigned" should have 1 issue (empty epic)
	if len(b.EpicColumns["Unassigned"]) != 1 {
		t.Errorf("Expected 1 issue in 'Unassigned' column, got %d", len(b.EpicColumns["Unassigned"]))
	}

	// Column order should be sorted by epic key, "Unassigned" last
	expectedOrder := []string{"PROJ-100", "PROJ-200", "Unassigned"}
	if len(b.ColumnOrder) != len(expectedOrder) {
		t.Errorf("Expected column order %v, got %v", expectedOrder, b.ColumnOrder)
	}

	// GroupMode should be GroupModeEpic
	if b.GroupMode != "epic" {
		t.Errorf("Expected GroupMode 'epic', got %q", b.GroupMode)
	}
}

func TestBoard_GroupByEpic_Empty(t *testing.T) {
	b := board.NewBoard()

	b.GroupByEpic([]*schema.Issue{})

	if len(b.EpicColumns) != 0 {
		t.Logf("Expected no columns, got %d", len(b.EpicColumns))
	}
}

func TestBoard_GroupByEpic_AllUnassigned(t *testing.T) {
	b := board.NewBoard()

	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-2",
				Type:    "bug",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Epic: "",
		},
	}

	b.GroupByEpic(issues)

	// All issues without epic go to "Unassigned"
	if len(b.EpicColumns["Unassigned"]) != 2 {
		t.Errorf("Expected 2 issues in 'Unassigned' column, got %d", len(b.EpicColumns["Unassigned"]))
	}
	if len(b.ColumnOrder) != 1 || b.ColumnOrder[0] != "Unassigned" {
		t.Errorf("Expected column order ['Unassigned'], got %v", b.ColumnOrder)
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