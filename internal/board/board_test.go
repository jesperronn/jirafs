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

func TestBoard_GroupByAssignee(t *testing.T) {
	board := NewBoard()

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

	board.GroupByAssignee(issues, users)

	// "712020:abcd" (Jesper) should have 2 issues
	assert.Len(t, board.AssigneeColumns["712020:abcd"], 2)

	// "712020:efgh" (Bob) should have 1 issue
	assert.Len(t, board.AssigneeColumns["712020:efgh"], 1)

	// "Unassigned" should have 1 issue
	assert.Len(t, board.AssigneeColumns["Unassigned"], 1)

	// Column order should be sorted by account ID, "Unassigned" last
	assert.Equal(t, []string{"712020:abcd", "712020:efgh", "Unassigned"}, board.ColumnOrder)

	// GroupMode should be GroupModeAssignee
	assert.Equal(t, GroupModeAssignee, board.GroupMode)
}

func TestBoard_GroupByAssignee_Empty(t *testing.T) {
	b := NewBoard()

	users := map[string]registry.User{
		"user:jesper": {AccountID: "712020:abcd"},
	}

	b.GroupByAssignee([]*schema.Issue{}, users)

	assert.Empty(t, b.AssigneeColumns)
	assert.Empty(t, b.ColumnOrder)
}

func TestBoard_GroupByAssignee_UnresolvedUser(t *testing.T) {
	b := NewBoard()

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
	assert.Len(t, b.AssigneeColumns["user:bob"], 1)
}

func TestBoard_GroupByEpic(t *testing.T) {
	board := NewBoard()

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

	board.GroupByEpic(issues)

	// "PROJ-100" should have 2 issues
	assert.Len(t, board.EpicColumns["PROJ-100"], 2)

	// "PROJ-200" should have 1 issue
	assert.Len(t, board.EpicColumns["PROJ-200"], 1)

	// "Unassigned" should have 1 issue
	assert.Len(t, board.EpicColumns["Unassigned"], 1)

	// Column order should be sorted by epic key, "Unassigned" last
	assert.Equal(t, []string{"PROJ-100", "PROJ-200", "Unassigned"}, board.ColumnOrder)

	// GroupMode should be GroupModeEpic
	assert.Equal(t, GroupModeEpic, board.GroupMode)
}

func TestBoard_GroupByEpic_Empty(t *testing.T) {
	b := NewBoard()

	b.GroupByEpic([]*schema.Issue{})

	assert.Empty(t, b.EpicColumns)
	assert.Empty(t, b.ColumnOrder)
}

func TestBoard_GroupByEpic_AllUnassigned(t *testing.T) {
	b := NewBoard()

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
	assert.Len(t, b.EpicColumns["Unassigned"], 2)
	assert.Equal(t, []string{"Unassigned"}, b.ColumnOrder)
}

func TestBoard_NewBoard_Defaults(t *testing.T) {
	board := NewBoard()

	assert.Equal(t, GroupModeStatus, board.GroupMode)
	assert.NotNil(t, board.StatusColumns)
	assert.NotNil(t, board.AssigneeColumns)
	assert.NotNil(t, board.EpicColumns)
	assert.NotNil(t, board.ColumnOrder)
}

func TestResolveAssigneeAccountID(t *testing.T) {
	users := map[string]registry.User{
		"user:jesper": {AccountID: "712020:abcd"},
		"user:bob":    {AccountID: "712020:efgh"},
	}

	// Nil assignee → "Unassigned"
	assert.Equal(t, "Unassigned", resolveAssigneeAccountID(nil, users))

	// Empty string assignee → "Unassigned"
	empty := ""
	assert.Equal(t, "Unassigned", resolveAssigneeAccountID(&empty, users))

	// Typed ref resolves to account ID
	assignee := "user:jesper"
	assert.Equal(t, "712020:abcd", resolveAssigneeAccountID(&assignee, users))

	// Typed ref not in registry → raw value
	unknown := "user:unknown"
	assert.Equal(t, "user:unknown", resolveAssigneeAccountID(&unknown, users))
}

func TestResolveEpicKey(t *testing.T) {
	// Empty epic → "Unassigned"
	assert.Equal(t, "Unassigned", resolveEpicKey(""))

	// Non-empty epic returns the key as-is
	assert.Equal(t, "PROJ-123", resolveEpicKey("PROJ-123"))
}

func TestGroupModes(t *testing.T) {
	assert.Equal(t, GroupModeStatus, GroupMode("status"))
	assert.Equal(t, GroupModeAssignee, GroupMode("assignee"))
	assert.Equal(t, GroupModeEpic, GroupMode("epic"))
}