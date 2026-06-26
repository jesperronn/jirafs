// Package board builds local kanban board views from mirror issue data.
package board

import (
	"sort"

	"github.com/jirafs/jirafs/internal/registry"
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

// GroupByStatus groups issues by their canonical status, using the status
// registry to resolve typed refs to status names. Columns are built from
// the registry entries, and issues whose status does not match any registry
// entry are placed in "Unassigned".
func (b *Board) GroupByStatus(issues []*schema.Issue, statuses map[string]registry.Status) {
	// Clear existing columns
	b.StatusColumns = make(map[string][]*schema.Issue)

	// Build column order from the registry: sorted by status name.
	columnOrder := []string{}
	for _, s := range statuses {
		if s.Name != "" {
			columnOrder = append(columnOrder, s.Name)
		}
	}
	sort.Strings(columnOrder)
	b.ColumnOrder = columnOrder

	// Group issues by resolved status name.
	for _, issue := range issues {
		statusName := resolveStatusName(issue.Status, statuses)
		b.StatusColumns[statusName] = append(b.StatusColumns[statusName], issue)
	}
}

// resolveStatusName resolves an issue's status field to a canonical name
// using the status registry. If the status is empty, returns "Unassigned".
// If the status is a typed ref (e.g. "status:in-progress"), resolves it
// via registry.ResolveStatus. If not found in the registry, returns the
// raw status value as-is.
func resolveStatusName(status string, statuses map[string]registry.Status) string {
	if status == "" {
		return "Unassigned"
	}

	// Try resolving as a typed ref first.
	name, found := registry.ResolveStatus(status, statuses)
	if found {
		return name
	}

	// Not found in registry — return the raw value.
	return status
}