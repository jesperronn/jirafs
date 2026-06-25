// Package board builds local kanban board views from mirror issue data.
package board

import (
	"sort"

	"github.com/jirafs/jirafs/internal/schema"
)

// Board represents a kanban board view grouped by status.
type Board struct {
	// StatusColumns maps canonical status names to lists of issues in that column.
	StatusColumns map[string][]*schema.Issue
	// ColumnOrder defines the canonical order of columns.
	ColumnOrder []string
	
	// AssigneeGroups maps assignees to lists of issues assigned to them.
	AssigneeGroups map[string][]*schema.Issue
	
	// EpicGroups maps epics to lists of issues belonging to them.
	EpicGroups map[string][]*schema.Issue
}

// NewBoard creates a new board grouping issues by status.
func NewBoard() *Board {
	return &Board{
		StatusColumns:  make(map[string][]*schema.Issue),
		ColumnOrder:    []string{},
		AssigneeGroups: make(map[string][]*schema.Issue),
		EpicGroups:     make(map[string][]*schema.Issue),
	}
}

// GroupByStatus groups issues by their canonical status.
func (b *Board) GroupByStatus(issues []*schema.Issue, statusRegistry interface{}) {
	// Clear existing columns
	b.StatusColumns = make(map[string][]*schema.Issue)
	
	// Initialize column order with default columns
	defaultColumns := []string{"Open", "In Progress", "Resolved", "Unknown"}
	
	// Create column order with canonical status names 
	columnOrder := []string{}
	
	// Add canonical columns in proper order
	for _, colName := range defaultColumns {
		columnOrder = append(columnOrder, colName)
	}
	
	b.ColumnOrder = columnOrder
	
	// Group issues by status
	for _, issue := range issues {
		statusName := getStatusForIssue(issue)
		
		// If status is not in our defined columns, put it in "Unknown"
		if !contains(b.ColumnOrder, statusName) {
			statusName = "Unknown"
		}
		
		b.StatusColumns[statusName] = append(b.StatusColumns[statusName], issue)
	}
	
	// Sort the columns in a consistent way (just for deterministic output)
	sort.Strings(b.ColumnOrder)
}

// GroupByAssignee groups issues by their assignee.
func (b *Board) GroupByAssignee(issues []*schema.Issue) {
	// Clear existing assignee groups
	b.AssigneeGroups = make(map[string][]*schema.Issue)
	
	// Group issues by assignee
	for _, issue := range issues {
		var assignee string
		
		if issue.Assignee != nil {
			assignee = *issue.Assignee
		} else {
			assignee = "Unassigned"
		}
		
		b.AssigneeGroups[assignee] = append(b.AssigneeGroups[assignee], issue)
	}
}

// GroupByEpic groups issues by their epic.
func (b *Board) GroupByEpic(issues []*schema.Issue) {
	// Clear existing epic groups
	b.EpicGroups = make(map[string][]*schema.Issue)
	
	// Group issues by epic
	for _, issue := range issues {
		var epic string
		
		// If there's an epic field in the issue, use that
		if issue.Epic != "" {
			epic = issue.Epic
		} else {
			epic = "No Epic"
		}
		
		b.EpicGroups[epic] = append(b.EpicGroups[epic], issue)
	}
}

// getStatusForIssue returns the canonical status name for an issue.
func getStatusForIssue(issue *schema.Issue) string {
	if issue.Status == "" {
		return "Open"
	}
	
	// For now, we return the raw status value as a canonical name
	return issue.Status
}

// contains checks if a string is in a slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}