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

func TestIssueToFieldsMap_empty(t *testing.T) {
	issue := Issue{}
	fields := issue.ToFieldsMap()
	if len(fields) != 0 {
		t.Errorf("expected empty fields map, got %d entries", len(fields))
	}
}

func TestIssueToFieldsMap_summaryOnly(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{Key: "PROJ-42"},
		Summary:  "Test summary",
	}
	fields := issue.ToFieldsMap()
	if fields["summary"] != "Test summary" {
		t.Errorf("summary = %v, want %q", fields["summary"], "Test summary")
	}
}

func TestIssueToFieldsMap_allFields(t *testing.T) {
	assignee := "jdoe"
	issue := Issue{
		Identity:    IssueIdentity{Key: "PROJ-42"},
		Summary:     "Test summary",
		Description: "Test description",
		Labels:      []string{"bug", "priority"},
		Assignee:    &assignee,
		Status:      "In Progress",
		Sprint:      "Sprint 42",
		FixVersions: []string{"1.0", "2.0"},
		LinkedIssues: []LinkedIssue{
			{Key: "PROJ-1", Type: "blocks"},
		},
	}
	fields := issue.ToFieldsMap()

	if fields["summary"] != "Test summary" {
		t.Errorf("summary = %v, want %q", fields["summary"], "Test summary")
	}
	if fields["description"] != "Test description" {
		t.Errorf("description = %v, want %q", fields["description"], "Test description")
	}
	labels, ok := fields["labels"].([]string)
	if !ok || len(labels) != 2 {
		t.Errorf("labels = %v, want 2 labels", fields["labels"])
	}
	assigneeField, ok := fields["assignee"].(map[string]string)
	if !ok || assigneeField["name"] != "jdoe" {
		t.Errorf("assignee = %v, want map with name=jdoe", fields["assignee"])
	}
	status, ok := fields["status"].(map[string]string)
	if !ok || status["name"] != "In Progress" {
		t.Errorf("status = %v, want map with name=In Progress", fields["status"])
	}
	sprint := fields["customfield_sprint"]
	if sprint != "Sprint 42" {
		t.Errorf("sprint = %v, want Sprint 42", sprint)
	}
	fv, ok := fields["fixVersions"].([]map[string]string)
	if !ok || len(fv) != 2 {
		t.Errorf("fixVersions = %v, want 2 versions", fields["fixVersions"])
	}
	links, ok := fields["issuelinks"].([]map[string]interface{})
	if !ok || len(links) != 1 {
		t.Errorf("issuelinks = %v, want 1 link", fields["issuelinks"])
	}
}

func TestIssueToFieldsMap_zeroValuesOmitted(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{Key: "PROJ-42"},
	}
	fields := issue.ToFieldsMap()
	if len(fields) != 0 {
		t.Errorf("expected empty fields map for zero-value issue, got %d entries", len(fields))
	}
}
