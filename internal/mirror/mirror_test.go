package mirror

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestImportReason_IsValid(t *testing.T) {
	for _, r := range ValidImportReasons {
		if !IsValidImportReason(r) {
			t.Errorf("IsValidImportReason(%q) = false, want true", r)
		}
	}
	if IsValidImportReason("bogus") {
		t.Error("IsValidImportReason(\"bogus\") = true, want false")
	}
	var zero ImportReason
	if IsValidImportReason(zero) {
		t.Error("IsValidImportReason(\"\") = true, want false")
	}
}

func TestImportedIssue_IsZero(t *testing.T) {
	var zero ImportedIssue
	if !zero.IsZero() {
		t.Error("zero ImportedIssue should be zero")
	}
	nonZero := ImportedIssue{Key: "PROJ-123", Reason: ImportReasonManual}
	if nonZero.IsZero() {
		t.Error("non-zero ImportedIssue should not be zero")
	}
	partial := ImportedIssue{Key: "PROJ-123"}
	if partial.IsZero() {
		t.Error("ImportedIssue with only Key set should not be zero")
	}
}

func TestImportedIssue_String(t *testing.T) {
	imp := ImportedIssue{Key: "PROJ-123", Reason: ImportReasonManual}
	got := imp.String()
	want := "PROJ-123 (manual)"
	if got != want {
		t.Errorf("ImportedIssue.String() = %q, want %q", got, want)
	}
}

func TestMirror_IsZero(t *testing.T) {
	var zero Mirror
	if !zero.IsZero() {
		t.Error("zero Mirror should be zero")
	}
	nonZero := Mirror{Project: schema.TypedRef{Type: schema.RefProject, Value: "ABC"}}
	if nonZero.IsZero() {
		t.Error("Mirror with project set should not be zero")
	}
	nonZero2 := Mirror{Issues: []ImportedIssue{{Key: "PROJ-123", Reason: ImportReasonManual}}}
	if nonZero2.IsZero() {
		t.Error("Mirror with issues set should not be zero")
	}
}

func TestMirror_HasIssue(t *testing.T) {
	m := Mirror{
		Project: schema.TypedRef{Type: schema.RefProject, Value: "ABC"},
		Issues: []ImportedIssue{
			{Key: "PROJ-123", Reason: ImportReasonManual},
			{Key: "PROJ-456", Reason: ImportReasonDependency},
		},
	}
	if !m.HasIssue("PROJ-123") {
		t.Error("mirror should have PROJ-123")
	}
	if !m.HasIssue("PROJ-456") {
		t.Error("mirror should have PROJ-456")
	}
	if m.HasIssue("PROJ-789") {
		t.Error("mirror should not have PROJ-789")
	}
}

func TestMirror_HasIssue_empty(t *testing.T) {
	var m Mirror
	if m.HasIssue("PROJ-123") {
		t.Error("empty mirror should not have any issues")
	}
	m.Issues = nil
	if m.HasIssue("PROJ-123") {
		t.Error("mirror with nil issues should not have any issues")
	}
}

func TestMirror_ImportReasonFor(t *testing.T) {
	m := Mirror{
		Issues: []ImportedIssue{
			{Key: "PROJ-123", Reason: ImportReasonManual},
			{Key: "PROJ-456", Reason: ImportReasonDependency},
		},
	}
	if got := m.ImportReasonFor("PROJ-123"); got != ImportReasonManual {
		t.Errorf("ImportReasonFor(PROJ-123) = %q, want %q", got, ImportReasonManual)
	}
	if got := m.ImportReasonFor("PROJ-456"); got != ImportReasonDependency {
		t.Errorf("ImportReasonFor(PROJ-456) = %q, want %q", got, ImportReasonDependency)
	}
	if got := m.ImportReasonFor("PROJ-789"); got != "" {
		t.Errorf("ImportReasonFor(PROJ-789) = %q, want \"\"", got)
	}
}

func TestMirror_ImportReasonFor_empty(t *testing.T) {
	var m Mirror
	if got := m.ImportReasonFor("PROJ-123"); got != "" {
		t.Errorf("empty mirror ImportReasonFor(PROJ-123) = %q, want \"\"", got)
	}
}
