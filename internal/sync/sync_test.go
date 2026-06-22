package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestSync_noOpPlanReturnsUnchangedRemote(t *testing.T) {
	// B063a: sync applies a validated no-op plan without mutation.
	localAssignee := "jdoe"
	remote := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"bug", "priority"},
		Assignee:    &localAssignee,
		Status:      "To Do",
		Sprint:      "Sprint 41",
		FixVersions: []string{"1.0"},
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1",
			ContentHash:   "abc123",
		},
	}
	plan := []schema.PlanOperation{} // no-op plan

	result := Sync(remote, remote, plan, t.TempDir())

	if result.Conflicts != nil {
		t.Errorf("expected no conflicts for no-op plan, got %v", result.Conflicts)
	}

	if result.Remote == nil {
		t.Fatal("expected non-nil remote in result")
	}

	if result.Remote.Summary != "Test issue" {
		t.Errorf("expected summary %q, got %q", "Test issue", result.Remote.Summary)
	}

	if result.Remote.Description != "Some description" {
		t.Errorf("expected description %q, got %q", "Some description", result.Remote.Description)
	}

	if result.Remote.Status != "To Do" {
		t.Errorf("expected status %q, got %q", "To Do", result.Remote.Status)
	}

	if result.Remote.Sprint != "Sprint 41" {
		t.Errorf("expected sprint %q, got %q", "Sprint 41", result.Remote.Sprint)
	}
}

func TestSync_nilPlanReturnsUnchangedRemote(t *testing.T) {
	// nil plan is treated as no-op.
	remote := schema.Issue{
		Summary:  "Test issue",
		Status:   "To Do",
		Sprint:   "Sprint 41",
		FixVersions: []string{"1.0"},
	}
	var plan []schema.PlanOperation // nil

	result := Sync(remote, remote, plan, t.TempDir())

	if result.Conflicts != nil {
		t.Errorf("expected no conflicts for nil plan, got %v", result.Conflicts)
	}

	if result.Remote == nil {
		t.Fatal("expected non-nil remote in result")
	}

	if result.Remote.Summary != "Test issue" {
		t.Errorf("expected summary %q, got %q", "Test issue", result.Remote.Summary)
	}
}

func TestSync_zeroLengthPlanReturnsUnchangedRemote(t *testing.T) {
	// zero-length (not nil) plan is treated as no-op.
	remote := schema.Issue{
		Summary: "Test issue",
		Status:  "To Do",
	}
	plan := []schema.PlanOperation{} // empty, not nil

	result := Sync(remote, remote, plan, t.TempDir())

	if result.Conflicts != nil {
		t.Errorf("expected no conflicts for zero-length plan, got %v", result.Conflicts)
	}

	if result.Remote == nil {
		t.Fatal("expected non-nil remote in result")
	}

	if result.Remote.Summary != "Test issue" {
		t.Errorf("expected summary %q, got %q", "Test issue", result.Remote.Summary)
	}
}

func TestSync_emptyIssueNoOp(t *testing.T) {
	// No-op plan with zero-value issues should still return a valid result.
	var local, remote schema.Issue
	plan := []schema.PlanOperation{}

	result := Sync(local, remote, plan, t.TempDir())

	if result.Conflicts != nil {
		t.Errorf("expected no conflicts for no-op with empty issues, got %v", result.Conflicts)
	}

	if result.Remote == nil {
		t.Fatal("expected non-nil remote in result")
	}
}

func TestSync_summaryChangeApplied(t *testing.T) {
	// Verify applyOp works for summary field.
	local := schema.Issue{Summary: "New summary"}
	remote := schema.Issue{Summary: "Old summary"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "New summary"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Summary != "New summary" {
		t.Errorf("expected summary %q, got %q", "New summary", result.Remote.Summary)
	}
}

func TestSync_descriptionChangeApplied(t *testing.T) {
	local := schema.Issue{Summary: "Test", Description: "New desc"}
	remote := schema.Issue{Summary: "Test", Description: "Old desc"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldDescription, Type: schema.OpSet, Value: "New desc"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Description != "New desc" {
		t.Errorf("expected description %q, got %q", "New desc", result.Remote.Description)
	}
}

func TestSync_labelsChangeApplied(t *testing.T) {
	local := schema.Issue{Summary: "Test", Labels: []string{"bug", "priority"}}
	remote := schema.Issue{Summary: "Test", Labels: []string{"bug"}}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldLabels, Type: schema.OpSet, Value: "bug,priority"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if len(result.Remote.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(result.Remote.Labels))
	}
}

func TestSync_assigneeChangeApplied(t *testing.T) {
	localAssignee := "jdoe"
	local := schema.Issue{Summary: "Test", Assignee: &localAssignee}
	remote := schema.Issue{Summary: "Test"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldAssignee, Type: schema.OpSet, Value: "jdoe"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Assignee == nil {
		t.Fatal("expected non-nil assignee")
	}
	if *result.Remote.Assignee != "jdoe" {
		t.Errorf("expected assignee %q, got %q", "jdoe", *result.Remote.Assignee)
	}
}

func TestSync_statusChangeRejectedBeforeMutation(t *testing.T) {
	local := schema.Issue{Summary: "Test", Status: "In Progress"}
	remote := schema.Issue{Summary: "Test", Status: "To Do"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldStatus, Type: schema.OpSet, Value: "In Progress"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected invalid transition conflict, got none")
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Conflicts[0].Type != schema.ConflictInvalidTransition {
		t.Fatalf("expected conflict type %q, got %q", schema.ConflictInvalidTransition, result.Conflicts[0].Type)
	}
	if result.Remote.Status != "To Do" {
		t.Errorf("expected status %q, got %q", "To Do", result.Remote.Status)
	}
}

func TestSync_sprintChangeApplied(t *testing.T) {
	local := schema.Issue{Summary: "Test", Sprint: "Sprint 42"}
	remote := schema.Issue{Summary: "Test", Sprint: "Sprint 41"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldSprint, Type: schema.OpSet, Value: "Sprint 42"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Sprint != "Sprint 42" {
		t.Errorf("expected sprint %q, got %q", "Sprint 42", result.Remote.Sprint)
	}
}

func TestSync_fixVersionsChangeApplied(t *testing.T) {
	local := schema.Issue{Summary: "Test", FixVersions: []string{"1.0", "2.0"}}
	remote := schema.Issue{Summary: "Test", FixVersions: []string{"1.0"}}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldFixVersions, Type: schema.OpSet, Value: "1.0,2.0"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if len(result.Remote.FixVersions) != 2 {
		t.Fatalf("expected 2 fix versions, got %d", len(result.Remote.FixVersions))
	}
}

func TestSync_splitValues(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"bug,priority", []string{"bug", "priority"}},
		{"single", []string{"single"}},
		{"", nil},
	}
	for _, tt := range tests {
		result := splitValues(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitValues(%q) got %d elements, expected %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitValues(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestSync_strPtr(t *testing.T) {
	val := "test"
	p := strPtr(val)
	if p == nil || *p != "test" {
		t.Errorf("strPtr(\"test\") = %v, want pointer to \"test\"", p)
	}
}

func TestSync_splitComma(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"", []string{""}},
	}
	for _, tt := range tests {
		result := splitComma(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitComma(%q) got %d elements, expected %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitComma(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestSync_archivePathInvalid_failsBeforeMutation(t *testing.T) {
	// B064a: invalid archive path produces conflict, not operations.
	local := schema.Issue{Summary: "Test issue"}
	remote := schema.Issue{Summary: "Old summary"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "Test issue"},
	}

	result := Sync(local, remote, plan, "/nonexistent/archive/path")

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflict for invalid archive path, got none")
	}

	conflict := result.Conflicts[0]
	if conflict.Type != schema.ConflictArchivePathInvalid {
		t.Errorf("expected conflict type %q, got %q", schema.ConflictArchivePathInvalid, conflict.Type)
	}

	// Remote should be unchanged.
	if result.Remote == nil {
		t.Fatal("expected non-nil remote in result")
	}
	if result.Remote.Summary != "Old summary" {
		t.Errorf("expected summary %q, got %q", "Old summary", result.Remote.Summary)
	}
}

func TestSync_archivePathIsFile_failsBeforeMutation(t *testing.T) {
	// B064a: archive path that is a file (not directory) produces conflict.
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not_a_dir")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()

	local := schema.Issue{Summary: "Test issue"}
	remote := schema.Issue{Summary: "Old summary"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "Test issue"},
	}

	result := Sync(local, remote, plan, filePath)

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflict for file-as-archive-path, got none")
	}

	conflict := result.Conflicts[0]
	if conflict.Type != schema.ConflictArchivePathInvalid {
		t.Errorf("expected conflict type %q, got %q", schema.ConflictArchivePathInvalid, conflict.Type)
	}
}

func TestSync_archivePathEmpty_failsBeforeMutation(t *testing.T) {
	// B064a: empty archive path produces conflict.
	local := schema.Issue{Summary: "Test issue"}
	remote := schema.Issue{Summary: "Old summary"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "Test issue"},
	}

	result := Sync(local, remote, plan, "")

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflict for empty archive path, got none")
	}

	conflict := result.Conflicts[0]
	if conflict.Type != schema.ConflictArchivePathInvalid {
		t.Errorf("expected conflict type %q, got %q", schema.ConflictArchivePathInvalid, conflict.Type)
	}
}

func TestSync_archivePathValid_allowsMutation(t *testing.T) {
	// B064a: valid archive path allows mutation to proceed.
	local := schema.Issue{Summary: "New summary"}
	remote := schema.Issue{Summary: "Old summary"}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "New summary"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Summary != "New summary" {
		t.Errorf("expected summary %q, got %q", "New summary", result.Remote.Summary)
	}
}

func TestSync_unresolvedLinkedIssue_failsBeforeMutation(t *testing.T) {
	// B064a: unresolved linked issue reference (empty key) produces conflict.
	local := schema.Issue{
		Summary:      "Test issue",
		LinkedIssues: []schema.LinkedIssue{{Key: "", Type: "blocks"}},
	}
	remote := schema.Issue{Summary: "Test issue"}
	plan := []schema.PlanOperation{}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflict for unresolved linked issue, got none")
	}

	conflict := result.Conflicts[0]
	if conflict.Type != schema.ConflictUnresolvedRef {
		t.Errorf("expected conflict type %q, got %q", schema.ConflictUnresolvedRef, conflict.Type)
	}
}

func TestSync_unresolvedRemoteLinkedIssue_failsBeforeMutation(t *testing.T) {
	// B064a: unresolved linked issue in remote produces conflict.
	local := schema.Issue{Summary: "Test issue"}
	remote := schema.Issue{
		Summary:      "Test issue",
		LinkedIssues: []schema.LinkedIssue{{Key: "", Type: "relates to"}},
	}
	plan := []schema.PlanOperation{}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflict for unresolved remote linked issue, got none")
	}

	conflict := result.Conflicts[0]
	if conflict.Type != schema.ConflictUnresolvedRef {
		t.Errorf("expected conflict type %q, got %q", schema.ConflictUnresolvedRef, conflict.Type)
	}
}

func TestSync_unresolvedAssignee_failsBeforeMutation(t *testing.T) {
	// B064a: unresolved assignee reference (empty value) produces conflict.
	emptyAssignee := ""
	local := schema.Issue{
		Summary:  "Test issue",
		Assignee: &emptyAssignee,
	}
	remote := schema.Issue{Summary: "Test issue"}
	plan := []schema.PlanOperation{}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflict for unresolved assignee, got none")
	}

	conflict := result.Conflicts[0]
	if conflict.Type != schema.ConflictUnresolvedRef {
		t.Errorf("expected conflict type %q, got %q", schema.ConflictUnresolvedRef, conflict.Type)
	}
}

func TestSync_unresolvedFixVersion_failsBeforeMutation(t *testing.T) {
	// B064a: unresolved fix version reference (empty value) produces conflict.
	local := schema.Issue{
		Summary:     "Test issue",
		FixVersions: []string{"1.0", ""},
	}
	remote := schema.Issue{Summary: "Test issue"}
	plan := []schema.PlanOperation{}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflict for unresolved fix version, got none")
	}

	conflict := result.Conflicts[0]
	if conflict.Type != schema.ConflictUnresolvedRef {
		t.Errorf("expected conflict type %q, got %q", schema.ConflictUnresolvedRef, conflict.Type)
	}
}

func TestSync_allRefsResolved_allowsMutation(t *testing.T) {
	// B064a: all references resolved allows mutation to proceed.
	assignee := "jdoe"
	local := schema.Issue{
		Summary:     "New summary",
		Assignee:    &assignee,
		FixVersions: []string{"1.0", "2.0"},
	}
	remote := schema.Issue{
		Summary:     "Old summary",
		Assignee:    &assignee,
		FixVersions: []string{"1.0", "2.0"},
	}
	plan := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "New summary"},
	}

	result := Sync(local, remote, plan, t.TempDir())

	if len(result.Conflicts) > 0 {
		t.Fatalf("unexpected conflicts: %v", result.Conflicts)
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Summary != "New summary" {
		t.Errorf("expected summary %q, got %q", "New summary", result.Remote.Summary)
	}
}

func TestSync_staleRemoteVersionConflictsBeforeMutation(t *testing.T) {
	local := schema.Issue{
		Summary: "New summary",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1",
			ContentHash:   "abc",
		},
	}
	remote := schema.Issue{
		Summary: "Old summary",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "2",
			ContentHash:   "abc",
		},
	}
	ops := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "New summary"},
	}

	result := Sync(local, remote, ops, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected stale-state conflict, got none")
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Summary != "Old summary" {
		t.Fatalf("expected remote to remain unchanged, got %q", result.Remote.Summary)
	}
}

func TestSync_staleContentHashConflictsBeforeMutation(t *testing.T) {
	local := schema.Issue{
		Summary: "New summary",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1",
			ContentHash:   "abc",
		},
	}
	remote := schema.Issue{
		Summary: "Old summary",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1",
			ContentHash:   "def",
		},
	}
	ops := []schema.PlanOperation{
		{Field: schema.EditableFieldSummary, Type: schema.OpSet, Value: "New summary"},
	}

	result := Sync(local, remote, ops, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected stale-state conflict, got none")
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Summary != "Old summary" {
		t.Fatalf("expected remote to remain unchanged, got %q", result.Remote.Summary)
	}
}

func TestSync_planMismatchConflictsBeforeMutation(t *testing.T) {
	local := schema.Issue{Summary: "Local summary"}
	remote := schema.Issue{Summary: "Remote summary"}
	ops := []schema.PlanOperation{
		{Field: schema.EditableFieldDescription, Type: schema.OpSet, Value: "wrong op"},
	}

	result := Sync(local, remote, ops, t.TempDir())

	if len(result.Conflicts) == 0 {
		t.Fatal("expected plan mismatch conflict, got none")
	}
	if result.Remote == nil {
		t.Fatal("expected non-nil remote")
	}
	if result.Remote.Summary != "Remote summary" {
		t.Fatalf("expected remote to remain unchanged, got %q", result.Remote.Summary)
	}
}
