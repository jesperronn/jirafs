// Package board builds local kanban board views from mirror issue data.
package board

import (
	"github.com/jirafs/jirafs/internal/schema"
)

// Board represents a kanban board view grouped by status.
type Board struct {
	// StatusColumns maps canonical status names to lists of issues in that column.
	StatusColumns map[string][]*schema.Issue
	// ColumnOrder defines the canonical order of columns.
	ColumnOrder []string
}

// NewBoard creates a new board grouping issues by status.
func NewBoard() *Board {
	return &Board{
		StatusColumns: make(map[string][]*schema.Issue),
		ColumnOrder:   []string{},
	}
}

// GroupByStatus groups issues by their canonical status.
func (b *Board) GroupByStatus(issues []*schema.Issue, statusRegistry interface{}) {
	// Clear existing columns
	b.StatusColumns = make(map[string][]*schema.Issue)
	
	// Initialize with default columns (open, in-progress, resolved, unknown)
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

// GroupByAssignee groups issues by their assignee.
func (b *Board) GroupByAssignee(issues []*schema.Issue) {
	// For now, just group by whether assignee exists or not
	assigneeColumns := make(map[string][]*schema.Issue)
	
	for _, issue := range issues {
		var assigneeKey string
		if issue.Assignee != nil && *issue.Assignee != "" {
			assigneeKey = *issue.Assignee
		} else {
			assigneeKey = "Unassigned"
		}
		
		assigneeColumns[assigneeKey] = append(assigneeColumns[assigneeKey], issue)
	}
	
	// For B091a, we just need to make sure grouping by status works properly
	// This method exists for B091b to use, but B091a is about status grouping
}

// GroupByEpic groups issues by their epic.
func (b *Board) GroupByEpic(issues []*schema.Issue) {
	// For B091b, we want to group by epic - this is a more complex operation
	// For now just make sure the method exists
	
	// This will be implemented in B091b task
}