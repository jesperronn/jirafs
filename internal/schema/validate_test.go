package schema

import "testing"

func TestIssueIdentity_ValidateRequired_allSet(t *testing.T) {
	id := IssueIdentity{
		Key:     "ABC-123",
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	errs := id.ValidateRequired()
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestIssueIdentity_ValidateRequired_missingKey(t *testing.T) {
	id := IssueIdentity{
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	errs := id.ValidateRequired()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "key" {
		t.Errorf("expected field 'key', got %q", errs[0].Field)
	}
}

func TestIssueIdentity_ValidateRequired_missingType(t *testing.T) {
	id := IssueIdentity{
		Key:     "ABC-123",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	errs := id.ValidateRequired()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "type" {
		t.Errorf("expected field 'type', got %q", errs[0].Field)
	}
}

func TestIssueIdentity_ValidateRequired_missingProject(t *testing.T) {
	id := IssueIdentity{
		Key: "ABC-123",
		Type: "story",
	}
	errs := id.ValidateRequired()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "project" {
		t.Errorf("expected field 'project', got %q", errs[0].Field)
	}
}

func TestIssueIdentity_ValidateRequired_allMissing(t *testing.T) {
	var id IssueIdentity
	errs := id.ValidateRequired()
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
	fields := []string{errs[0].Field, errs[1].Field, errs[2].Field}
	expected := []string{"key", "type", "project"}
	for i, want := range expected {
		if fields[i] != want {
			t.Errorf("error[%d] field = %q, want %q", i, fields[i], want)
		}
	}
}

func TestIssueIdentity_IsComplete(t *testing.T) {
	complete := IssueIdentity{
		Key:     "ABC-123",
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	if !complete.IsComplete() {
		t.Error("complete identity should report IsComplete")
	}

	incomplete := IssueIdentity{}
	if incomplete.IsComplete() {
		t.Error("empty identity should not report IsComplete")
	}
}

func TestValidateSections_allKnown(t *testing.T) {
	sections := []FixedSectionName{
		SecDescription,
		SecAcceptanceCriteria,
		SecNotes,
	}
	errs := ValidateSections(sections)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateSections_oneUnknown(t *testing.T) {
	sections := []FixedSectionName{
		SecDescription,
		FixedSectionName("Custom Section"),
	}
	errs := ValidateSections(sections)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "section" {
		t.Errorf("expected field 'section', got %q", errs[0].Field)
	}
}

func TestValidateSections_allUnknown(t *testing.T) {
	sections := []FixedSectionName{
		FixedSectionName("Foo"),
		FixedSectionName("Bar"),
	}
	errs := ValidateSections(sections)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateSections_empty(t *testing.T) {
	errs := ValidateSections(nil)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for nil, got %v", errs)
	}

	errs = ValidateSections([]FixedSectionName{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors for empty, got %v", errs)
	}
}

func TestValidateSections_emptySectionName(t *testing.T) {
	sections := []FixedSectionName{
		SecDescription,
		FixedSectionName(""),
	}
	errs := ValidateSections(sections)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for empty section, got %d: %v", len(errs), errs)
	}
}

func TestValidationError_Error(t *testing.T) {
	e := ValidationError{Field: "key", Msg: "is required"}
	got := e.Error()
	want := "key: is required"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
