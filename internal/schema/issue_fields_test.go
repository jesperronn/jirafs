package schema

import (
	"testing"
)

// B021b: tests for editable issue fields and fixed section names.

func TestEditableFieldStatus(t *testing.T) {
	if EditableFieldStatus != "status" {
		t.Errorf("EditableFieldStatus = %q, want %q", EditableFieldStatus, "status")
	}
}

func TestEditableFieldSprint(t *testing.T) {
	if EditableFieldSprint != "sprint" {
		t.Errorf("EditableFieldSprint = %q, want %q", EditableFieldSprint, "sprint")
	}
}

func TestEditableFieldFixVersions(t *testing.T) {
	if EditableFieldFixVersions != "fix_versions" {
		t.Errorf("EditableFieldFixVersions = %q, want %q", EditableFieldFixVersions, "fix_versions")
	}
}

func TestEditableFieldParent(t *testing.T) {
	if EditableFieldParent != "parent" {
		t.Errorf("EditableFieldParent = %q, want %q", EditableFieldParent, "parent")
	}
}

func TestEditableFieldEpic(t *testing.T) {
	if EditableFieldEpic != "epic" {
		t.Errorf("EditableFieldEpic = %q, want %q", EditableFieldEpic, "epic")
	}
}

func TestEditableFieldSummary(t *testing.T) {
	if EditableFieldSummary != "summary" {
		t.Errorf("EditableFieldSummary = %q, want %q", EditableFieldSummary, "summary")
	}
}

func TestEditableFieldDescription(t *testing.T) {
	if EditableFieldDescription != "description" {
		t.Errorf("EditableFieldDescription = %q, want %q", EditableFieldDescription, "description")
	}
}

func TestEditableFieldLabels(t *testing.T) {
	if EditableFieldLabels != "labels" {
		t.Errorf("EditableFieldLabels = %q, want %q", EditableFieldLabels, "labels")
	}
}

func TestEditableFieldAssignee(t *testing.T) {
	if EditableFieldAssignee != "assignee" {
		t.Errorf("EditableFieldAssignee = %q, want %q", EditableFieldAssignee, "assignee")
	}
}

func TestAllEditableFieldConstantsComplete(t *testing.T) {
	// Verify all 9 editable field constants are accounted for.
	all := []EditableField{
		EditableFieldSummary,
		EditableFieldDescription,
		EditableFieldLabels,
		EditableFieldAssignee,
		EditableFieldStatus,
		EditableFieldSprint,
		EditableFieldFixVersions,
		EditableFieldParent,
		EditableFieldEpic,
	}
	if len(all) != 9 {
		t.Fatalf("expected 9 editable field constants, got %d", len(all))
	}
	// Verify no duplicates.
	seen := make(map[EditableField]bool)
	for _, ef := range all {
		if seen[ef] {
			t.Errorf("duplicate editable field constant: %q", ef)
		}
		seen[ef] = true
	}
}

func TestPermissionCategoryEditable(t *testing.T) {
	if PermissionEditable != "editable" {
		t.Errorf("PermissionEditable = %q, want %q", PermissionEditable, "editable")
	}
}

func TestPermissionCategoryAppendOnly(t *testing.T) {
	if PermissionAppendOnly != "append_only" {
		t.Errorf("PermissionAppendOnly = %q, want %q", PermissionAppendOnly, "append_only")
	}
}

func TestPermissionCategoryReadOnly(t *testing.T) {
	if PermissionReadOnly != "read_only" {
		t.Errorf("PermissionReadOnly = %q, want %q", PermissionReadOnly, "read_only")
	}
}

func TestPermissionModel_ZeroValue(t *testing.T) {
	var pm PermissionModel
	if len(pm.Editable) != 0 {
		t.Errorf("zero PermissionModel.Editable = %v, want empty", pm.Editable)
	}
	if len(pm.AppendOnly) != 0 {
		t.Errorf("zero PermissionModel.AppendOnly = %v, want empty", pm.AppendOnly)
	}
	if len(pm.ReadOnly) != 0 {
		t.Errorf("zero PermissionModel.ReadOnly = %v, want empty", pm.ReadOnly)
	}
}

func TestPermissionModel_NonZero(t *testing.T) {
	pm := PermissionModel{
		Editable:   []EditableField{EditableFieldSummary, EditableFieldStatus},
		AppendOnly: []string{"comments", "resolution"},
		ReadOnly:   []string{"created", "updated"},
	}
	if len(pm.Editable) != 2 {
		t.Errorf("Editable = %d, want 2", len(pm.Editable))
	}
	if len(pm.AppendOnly) != 2 {
		t.Errorf("AppendOnly = %d, want 2", len(pm.AppendOnly))
	}
	if len(pm.ReadOnly) != 2 {
		t.Errorf("ReadOnly = %d, want 2", len(pm.ReadOnly))
	}
}

func TestFixedSectionName_Description(t *testing.T) {
	if SecDescription != "Description" {
		t.Errorf("SecDescription = %q, want %q", SecDescription, "Description")
	}
}

func TestFixedSectionName_AcceptanceCriteria(t *testing.T) {
	if SecAcceptanceCriteria != "Acceptance Criteria" {
		t.Errorf("SecAcceptanceCriteria = %q, want %q", SecAcceptanceCriteria, "Acceptance Criteria")
	}
}

func TestFixedSectionName_DefinitionOfReady(t *testing.T) {
	if SecDefinitionOfReady != "Definition of Ready" {
		t.Errorf("SecDefinitionOfReady = %q, want %q", SecDefinitionOfReady, "Definition of Ready")
	}
}

func TestFixedSectionName_Notes(t *testing.T) {
	if SecNotes != "Notes" {
		t.Errorf("SecNotes = %q, want %q", SecNotes, "Notes")
	}
}

func TestFixedSectionName_CommentsToAdd(t *testing.T) {
	if SecCommentsToAdd != "Comments To Add" {
		t.Errorf("SecCommentsToAdd = %q, want %q", SecCommentsToAdd, "Comments To Add")
	}
}

func TestFixedSectionName_RemoteComments(t *testing.T) {
	if SecRemoteComments != "Remote Comments" {
		t.Errorf("SecRemoteComments = %q, want %q", SecRemoteComments, "Remote Comments")
	}
}

func TestAllFixedSectionConstantsComplete(t *testing.T) {
	// Verify all 6 fixed section name constants are accounted for.
	all := []FixedSectionName{
		SecDescription,
		SecAcceptanceCriteria,
		SecDefinitionOfReady,
		SecNotes,
		SecCommentsToAdd,
		SecRemoteComments,
	}
	if len(all) != 6 {
		t.Fatalf("expected 6 fixed section name constants, got %d", len(all))
	}
	// Verify no duplicates.
	seen := make(map[FixedSectionName]bool)
	for _, fs := range all {
		if seen[fs] {
			t.Errorf("duplicate fixed section name constant: %q", fs)
		}
		seen[fs] = true
	}
}
