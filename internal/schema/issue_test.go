package schema

import "testing"

func TestIssueIdentity_IsZero(t *testing.T) {
	var zero IssueIdentity
	if !zero.IsZero() {
		t.Error("zero IssueIdentity should be IsZero")
	}

	nonZero := IssueIdentity{
		Key:     "ABC-123",
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	if nonZero.IsZero() {
		t.Error("non-zero IssueIdentity should not be IsZero")
	}

	partial := IssueIdentity{Key: "ABC-123"}
	if partial.IsZero() {
		t.Error("partial IssueIdentity should not be IsZero")
	}
}

func TestMachineOwned_IsZero(t *testing.T) {
	var zero MachineOwned
	if !zero.IsZero() {
		t.Error("zero MachineOwned should be IsZero")
	}

	nonZero := MachineOwned{SchemaVersion: "1"}
	if nonZero.IsZero() {
		t.Error("non-zero MachineOwned should not be IsZero")
	}
}

func TestEditableFieldConstants(t *testing.T) {
	expected := []EditableField{
		EditableFieldSummary,
		EditableFieldDescription,
		EditableFieldLabels,
		EditableFieldAssignee,
		EditableFieldParent,
		EditableFieldEpic,
		EditableFieldFixVersions,
		EditableFieldSprint,
	}
	for _, ef := range expected {
		if ef == "" {
			t.Errorf("editable field should not be empty: %q", ef)
		}
	}
}

func TestPermissionModel(t *testing.T) {
	pm := PermissionModel{
		Editable:   []EditableField{EditableFieldSummary, EditableFieldDescription},
		AppendOnly: []string{"comments"},
		ReadOnly:   []string{"reporter", "created"},
	}

	if len(pm.Editable) != 2 {
		t.Errorf("expected 2 editable fields, got %d", len(pm.Editable))
	}
	if len(pm.AppendOnly) != 1 {
		t.Errorf("expected 1 append-only field, got %d", len(pm.AppendOnly))
	}
	if len(pm.ReadOnly) != 2 {
		t.Errorf("expected 2 read-only fields, got %d", len(pm.ReadOnly))
	}
}

func TestFixedSectionName_IsKnown(t *testing.T) {
	known := []FixedSectionName{
		SecDescription,
		SecAcceptanceCriteria,
		SecDefinitionOfReady,
		SecNotes,
		SecCommentsToAdd,
		SecRemoteComments,
	}
	for _, fs := range known {
		if !fs.IsKnown() {
			t.Errorf("%q should be a known section", fs)
		}
	}

	unknown := FixedSectionName("Unknown Section")
	if unknown.IsKnown() {
		t.Error("unknown section should not be known")
	}

	empty := FixedSectionName("")
	if empty.IsKnown() {
		t.Error("empty section should not be known")
	}
}

func TestAllFixedSections(t *testing.T) {
	sections := AllFixedSections()
	if len(sections) != 6 {
		t.Fatalf("expected 6 fixed sections, got %d", len(sections))
	}

	expected := []FixedSectionName{
		SecDescription,
		SecAcceptanceCriteria,
		SecDefinitionOfReady,
		SecNotes,
		SecCommentsToAdd,
		SecRemoteComments,
	}
	for i, want := range expected {
		if sections[i] != want {
			t.Errorf("section[%d] = %q, want %q", i, sections[i], want)
		}
	}
}
