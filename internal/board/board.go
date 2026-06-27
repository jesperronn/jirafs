// Package board builds local kanban board views from mirror issue data.
package board

import (
	"sort"

	"github.com/jirafs/jirafs/internal/registry"
	"github.com/jirafs/jirafs/internal/schema"
)

// GroupMode defines the grouping strategy for a board.
type GroupMode string

const (
	// GroupModeStatus groups issues by their status.
	GroupModeStatus GroupMode = "status"
	// GroupModeAssignee groups issues by their assignee.
	GroupModeAssignee GroupMode = "assignee"
	// GroupModeEpic groups issues by their epic.
	GroupModeEpic GroupMode = "epic"
)

// Board represents a kanban board view with stable grouping keys for
// status, assignee, and epic. The active grouping is determined by
// GroupMode.
type Board struct {
	// GroupMode defines the active grouping strategy.
	GroupMode GroupMode
	// StatusColumns maps canonical status names to lists of issues in that column.
	StatusColumns map[string][]*schema.Issue
	// AssigneeColumns maps assignee account IDs to lists of issues.
	AssigneeColumns map[string][]*schema.Issue
	// EpicColumns maps epic keys to lists of issues.
	EpicColumns map[string][]*schema.Issue
	// ColumnOrder defines the canonical order of columns.
	ColumnOrder []string
}

// NewBoard creates a new board with a default grouping mode of status.
func NewBoard() *Board {
	return &Board{
		GroupMode:       GroupModeStatus,
		StatusColumns:   make(map[string][]*schema.Issue),
		AssigneeColumns: make(map[string][]*schema.Issue),
		EpicColumns:     make(map[string][]*schema.Issue),
		ColumnOrder:     []string{},
	}
}

// GroupByStatus groups issues by their canonical status, using the status
// registry to resolve typed refs to status names. Columns are built from
// the registry entries, and issues whose status does not match any registry
// entry are placed in "Unassigned".
func (b *Board) GroupByStatus(issues []*schema.Issue, statuses map[string]registry.Status) {
	b.GroupMode = GroupModeStatus

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

	// If registry is empty, build column order from actual statuses found in issues.
	if len(statuses) == 0 && len(b.StatusColumns) > 0 {
		b.ColumnOrder = []string{}
		for statusName := range b.StatusColumns {
			b.ColumnOrder = append(b.ColumnOrder, statusName)
		}
		sort.Strings(b.ColumnOrder)
	}
}

// GroupByAssignee groups issues by their assignee, using the user registry
// to resolve typed refs to stable account IDs. Issues without an assignee
// are placed under "Unassigned". Assignee columns are sorted by account ID
// deterministically.
func (b *Board) GroupByAssignee(issues []*schema.Issue, users map[string]registry.User) {
	b.GroupMode = GroupModeAssignee

	// Clear existing columns
	b.AssigneeColumns = make(map[string][]*schema.Issue)

	// Group issues by resolved assignee account ID.
	for _, issue := range issues {
		accountID := resolveAssigneeAccountID(issue.Assignee, users)
		b.AssigneeColumns[accountID] = append(b.AssigneeColumns[accountID], issue)
	}

	// Build column order: sorted by account ID, then "Unassigned" last.
	columnOrder := []string{}
	for id := range b.AssigneeColumns {
		if id != "Unassigned" {
			columnOrder = append(columnOrder, id)
		}
	}
	sort.Strings(columnOrder)
	// Append "Unassigned" last if it exists.
	if _, ok := b.AssigneeColumns["Unassigned"]; ok {
		columnOrder = append(columnOrder, "Unassigned")
	}
	b.ColumnOrder = columnOrder
}

// GroupByEpic groups issues by their epic key. Issues without an epic
// are placed under "Unassigned". Epic columns are sorted by epic key
// deterministically.
func (b *Board) GroupByEpic(issues []*schema.Issue) {
	b.GroupMode = GroupModeEpic

	// Clear existing columns
	b.EpicColumns = make(map[string][]*schema.Issue)

	// Group issues by epic key.
	for _, issue := range issues {
		epKey := resolveEpicKey(issue.Epic)
		b.EpicColumns[epKey] = append(b.EpicColumns[epKey], issue)
	}

	// Build column order: sorted by epic key, then "Unassigned" last.
	columnOrder := []string{}
	for ep := range b.EpicColumns {
		if ep != "Unassigned" {
			columnOrder = append(columnOrder, ep)
		}
	}
	sort.Strings(columnOrder)
	// Append "Unassigned" last if it exists.
	if _, ok := b.EpicColumns["Unassigned"]; ok {
		columnOrder = append(columnOrder, "Unassigned")
	}
	b.ColumnOrder = columnOrder
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

// resolveAssigneeAccountID resolves an issue's assignee field to a stable
// account ID using the user registry. If the assignee is nil, returns
// "Unassigned". If the assignee is a typed ref (e.g. "user:jesper"),
// resolves it via registry.ResolveUser. If not found in the registry,
// falls back to the raw assignee name as-is.
func resolveAssigneeAccountID(assignee *string, users map[string]registry.User) string {
	if assignee == nil || *assignee == "" {
		return "Unassigned"
	}

	// Try resolving as a typed ref first.
	accountID, found := registry.ResolveUser(*assignee, users)
	if found {
		return accountID
	}

	// Not found in registry — use the raw assignee name.
	return *assignee
}

// resolveEpicKey resolves an issue's epic field to a stable epic key.
// If the epic is empty, returns "Unassigned". The epic key is the
// issue key of the parent epic (e.g. "PROJ-123").
func resolveEpicKey(epic string) string {
	if epic == "" {
		return "Unassigned"
	}

	return epic
}